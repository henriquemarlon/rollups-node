// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// Algorithm for the state transition of computed claims. Possible actions are:
// - update epoch in the database
// - submit claim to blockchain
// - transition application to an invalid state
//
// 1. On startup of a clean blockchain there are no previous claims nor events.
//
//   - This configuration must submit a new computed claim.
//
//     2. Some time after the submission, the computed claim shows up as a claimSubmitted
//     event in the blockchain. The claim and event must match.
//
//   - This configuration must update the epoch in the database: computed -> submitted
//
// 3. After the first epoch, additional checks must be done. Same as (1) otherwise.
// 3.1. No epoch was skipped:
//   - previous_claim.last_block < current_claim.first_block
//
// 4. After the first epoch, additional checks must be done. Same as (2) otherwise.
// 4.1. epochs are in order:
//   - previous_claim.last_block < current_claim.first_block
//
// 4.2. There are no events between the epochs
//   - next(previous_event) == current_event
//
// Other cases are errors.
//
// | n |      prev     |      curr     | action |
// |   | claim | event | claim | event |        |
// |---+-------+-------+-------+-------+--------+
// | 1 |   .   |   .   |  cc   |   .   | submit |
// | 2 |   .   |   .   |  cc   |  ce   | update |
// | 3 |  pc   |  pe   |  cc   |   .   | submit |
// | 4 |  pc   |  pe   |  cc   |  ce   | update |
package claimer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrClaimMismatch = fmt.Errorf("Claim and antecessor mismatch")
	ErrEventMismatch = fmt.Errorf("Computed Claim mismatches ClaimSubmitted event")
	ErrMissingEvent  = fmt.Errorf("Accepted claim has no matching blockchain event")
)

type iclaimerRepository interface {
	ListApplications(ctx context.Context, f repository.ApplicationFilter, p repository.Pagination) ([]*Application, uint64, error)

	SelectSubmittedClaimPairsPerApp(ctx context.Context) (
		map[common.Address]*ClaimRow,
		map[common.Address]*ClaimRow,
		error,
	)
	SelectAcceptedClaimPairsPerApp(ctx context.Context) (
		map[common.Address]*ClaimRow,
		map[common.Address]*ClaimRow,
		error,
	)
	UpdateEpochWithSubmittedClaim(
		ctx context.Context,
		application_id int64,
		index uint64,
		transaction_hash common.Hash,
	) error
	UpdateEpochWithAcceptedClaim(
		ctx context.Context,
		application_id int64,
		index uint64,
	) error

	UpdateApplicationState(
		ctx context.Context,
		appID int64,
		state ApplicationState,
		reason *string,
	) error

	SaveNodeConfigRaw(ctx context.Context, key string, rawJSON []byte) error
	LoadNodeConfigRaw(ctx context.Context, key string) (rawJSON []byte, createdAt, updatedAt time.Time, err error)
}

func (s *Service) checkApplicationConsensus(endBlock *big.Int) []error {
	f := repository.ApplicationFilter{
		State: Pointer(ApplicationState_Enabled),
	}
	apps, _, err := s.repository.ListApplications(s.Context, f, repository.Pagination{})
	if err != nil {
		s.Logger.Error("Error retrieving enabled applications", "error", err)
		return []error{err}
	}

	var errs []error
	changedList, errs := s.blockchain.checkApplicationsForConsensusAddressChange(s.Context, apps, endBlock)
	for _, changed := range changedList {
		err = s.setApplicationInoperable(
			s.Context,
			changed.application.IApplicationAddress,
			changed.application.ID,
			"Application consensus address has changed. Application: %v, previous: %v, current: %v.",
			changed.application.IApplicationAddress,
			changed.application.IConsensusAddress,
			changed.newAddress,
		)
	}
	return errs
}

/* transition claims from computed to submitted */
func (s *Service) submitClaimsAndUpdateDatabase(endBlock *big.Int) []error {
	errs := []error{}
	acceptedOrSubmittedClaims, computedClaims, err := s.repository.SelectSubmittedClaimPairsPerApp(s.Context)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	// check claims in flight
	for key, txHash := range s.claimsInFlight {
		ready, receipt, err := s.blockchain.pollTransaction(s.Context, txHash, endBlock)
		if err != nil {
			s.Logger.Warn("Claim submission failed, retrying.",
				"txHash", txHash,
				"err", err,
			)
			delete(s.claimsInFlight, key)
			continue
		}
		if !ready {
			continue
		}
		if claim, ok := computedClaims[key]; ok {
			err = s.repository.UpdateEpochWithSubmittedClaim(s.Context, claim.ApplicationID, claim.Index, receipt.TxHash)
			if err != nil {
				errs = append(errs, err)
				return errs
			}
			s.Logger.Info("Claim submitted",
				"app", claim.IApplicationAddress,
				"receipt_block_number", receipt.BlockNumber,
				"claim_hash", fmt.Sprintf("%x", claim.ClaimHash),
				"last_block", claim.LastBlock,
				"tx", txHash)
			delete(computedClaims, key)
		} else {
			s.Logger.Warn("expected claim in flight to be in currClaims.",
				"tx", receipt.TxHash)
		}
		delete(s.claimsInFlight, key)
	}

	// check computed claims
	for key, computedClaim := range computedClaims {
		var ic *iconsensus.IConsensus
		var prevEvent *iconsensus.IConsensusClaimSubmitted
		var currEvent *iconsensus.IConsensusClaimSubmitted

		if _, isInFlight := s.claimsInFlight[key]; isInFlight {
			continue
		}

		prevClaimRow, prevExists := acceptedOrSubmittedClaims[key]
		if prevExists {
			err := checkClaimsConstraint(prevClaimRow, computedClaim)
			if err != nil {
				err = s.setApplicationInoperable(
					s.Context,
					prevClaimRow.IApplicationAddress,
					prevClaimRow.ApplicationID,
					"database mismatch on epochs. application: %v, epochs: %v (%v), %v (%v).",
					prevClaimRow.IApplicationAddress,
					prevClaimRow.Index,
					prevClaimRow.VirtualIndex,
					computedClaim.Index,
					computedClaim.VirtualIndex,
				)
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}

			// if prevClaimRow exists, there must be a matching event
			ic, prevEvent, currEvent, err =
				s.blockchain.findClaimSubmittedEventAndSucc(s.Context, prevClaimRow, endBlock)
			if err != nil {
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			if prevEvent == nil {
				err = s.setApplicationInoperable(
					s.Context,
					prevClaimRow.IApplicationAddress,
					prevClaimRow.ApplicationID,
					"epoch has no matching event. application: %v, epoch: %v (%v).",
					prevClaimRow.IApplicationAddress,
					prevClaimRow.Index,
					prevClaimRow.VirtualIndex,
				)
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			if !claimSubmittedMatch(prevClaimRow, prevEvent) {
				s.Logger.Error("event mismatch",
					"claim", prevClaimRow,
					"event", prevEvent,
					"err", ErrEventMismatch,
				)
				err = s.setApplicationInoperable(
					s.Context,
					prevClaimRow.IApplicationAddress,
					prevClaimRow.ApplicationID,
					"epoch has an invalid event: %v, epoch: %v (%v). event: %v",
					computedClaim.Index,
					prevClaimRow.Index,
					prevClaimRow.VirtualIndex,
					prevEvent,
				)
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
		} else {
			// first claim
			ic, currEvent, _, err =
				s.blockchain.findClaimSubmittedEventAndSucc(s.Context, computedClaim, endBlock)
			if err != nil {
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
		}

		if currEvent != nil {
			s.Logger.Debug("Found ClaimSubmitted Event",
				"app", currEvent.AppContract,
				"claim_hash", fmt.Sprintf("%x", currEvent.OutputsMerkleRoot),
				"last_block", currEvent.LastProcessedBlockNumber.Uint64(),
			)
			if !claimSubmittedMatch(computedClaim, currEvent) {
				err = s.setApplicationInoperable(
					s.Context,
					computedClaim.IApplicationAddress,
					computedClaim.ApplicationID,
					"computed claim does not match event. computed_claim=%v, current_event=%v",
					computedClaim, currEvent,
				)
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			s.Logger.Debug("Updating claim status to submitted",
				"app", computedClaim.IApplicationAddress,
				"claim_hash", fmt.Sprintf("%x", computedClaim.ClaimHash),
				"last_block", computedClaim.LastBlock,
			)
			txHash := currEvent.Raw.TxHash
			err = s.repository.UpdateEpochWithSubmittedClaim(
				s.Context,
				computedClaim.ApplicationID,
				computedClaim.Index,
				txHash,
			)
			if err != nil {
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			delete(s.claimsInFlight, key)
			s.Logger.Info("Claim previously submitted",
				"app", computedClaim.IApplicationAddress,
				"event_block_number", currEvent.Raw.BlockNumber,
				"claim_hash", fmt.Sprintf("%x", computedClaim.ClaimHash),
				"last_block", computedClaim.LastBlock,
			)
		} else if s.submissionEnabled {
			if prevClaimRow != nil && prevClaimRow.Status != EpochStatus_ClaimAccepted {
				s.Logger.Debug("Waiting previous claim to be accepted before submitting new one. Previous:",
					"app", prevClaimRow.IApplicationAddress,
					"claim_hash", fmt.Sprintf("%x", prevClaimRow.ClaimHash),
					"last_block", prevClaimRow.LastBlock,
				)
				goto nextApp
			}
			s.Logger.Debug("Submitting claim to blockchain",
				"app", computedClaim.IApplicationAddress,
				"claim_hash", fmt.Sprintf("%x", computedClaim.ClaimHash),
				"last_block", computedClaim.LastBlock,
			)
			txHash, err := s.blockchain.submitClaimToBlockchain(ic, computedClaim)
			if err != nil {
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			s.claimsInFlight[key] = txHash
		} else {
			s.Logger.Debug("Claim submission disabled. Doing nothing",
				"app", computedClaim.IApplicationAddress,
				"claim_hash", fmt.Sprintf("%x", computedClaim.ClaimHash),
				"last_block", computedClaim.LastBlock,
			)

		}
	nextApp:
	}
	return errs
}

/* transition claims from submitted to accepted */
func (s *Service) acceptClaimsAndUpdateDatabase(endBlock *big.Int) []error {
	errs := []error{}
	acceptedClaims, submittedClaims, err := s.repository.SelectAcceptedClaimPairsPerApp(s.Context)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	// check submitted claims
	for key, submittedClaim := range submittedClaims {
		var prevEvent *iconsensus.IConsensusClaimAccepted
		var currEvent *iconsensus.IConsensusClaimAccepted

		acceptedClaim, prevExists := acceptedClaims[key]
		if prevExists {
			err := checkClaimsConstraint(acceptedClaim, submittedClaim)
			if err != nil {
				s.Logger.Error("database mismatch",
					"prevClaim", acceptedClaim,
					"currClaim", submittedClaim,
					"err", err,
				)
				delete(submittedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}

			// if prevClaimRow exists, there must be a matching event
			_, prevEvent, currEvent, err =
				s.blockchain.findClaimAcceptedEventAndSucc(s.Context, acceptedClaim, endBlock)
			if err != nil {
				delete(submittedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			if prevEvent == nil {
				s.Logger.Error("missing event",
					"claim", acceptedClaim,
					"err", ErrMissingEvent,
				)
				delete(submittedClaims, key)
				errs = append(errs, ErrMissingEvent)
				goto nextApp
			}
			if !claimAcceptedMatch(acceptedClaim, prevEvent) {
				s.Logger.Error("event mismatch",
					"claim", acceptedClaim,
					"event", prevEvent,
					"err", ErrEventMismatch,
				)
				delete(submittedClaims, key)
				errs = append(errs, ErrEventMismatch)
				goto nextApp
			}
		} else {
			// first claim
			_, currEvent, _, err =
				s.blockchain.findClaimAcceptedEventAndSucc(s.Context, submittedClaim, endBlock)
			if err != nil {
				delete(submittedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
		}

		if currEvent != nil {
			s.Logger.Debug("Found ClaimAccepted Event",
				"app", currEvent.AppContract,
				"claim_hash", fmt.Sprintf("%x", currEvent.OutputsMerkleRoot),
				"last_block", currEvent.LastProcessedBlockNumber.Uint64(),
			)
			if !claimAcceptedMatch(submittedClaim, currEvent) {
				s.Logger.Error("event mismatch",
					"claim", submittedClaim,
					"event", currEvent,
					"err", ErrEventMismatch,
				)
				delete(submittedClaims, key)
				errs = append(errs, ErrEventMismatch)
				goto nextApp
			}
			s.Logger.Debug("Updating claim status to accepted",
				"app", submittedClaim.IApplicationAddress,
				"claim_hash", fmt.Sprintf("%x", submittedClaim.ClaimHash),
				"last_block", submittedClaim.LastBlock,
			)
			txHash := currEvent.Raw.TxHash
			err = s.repository.UpdateEpochWithAcceptedClaim(s.Context, submittedClaim.ApplicationID, submittedClaim.Index)
			if err != nil {
				delete(submittedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			s.Logger.Info("Claim accepted",
				"app", currEvent.AppContract,
				"event_block_number", currEvent.Raw.BlockNumber,
				"claim_hash", fmt.Sprintf("%x", currEvent.OutputsMerkleRoot),
				"last_block", currEvent.LastProcessedBlockNumber.Uint64(),
				"tx", txHash,
			)
		}
	nextApp:
	}
	return errs
}

// setApplicationInoperable marks an application as inoperable with the given reason,
// logs any error that occurs during the update, and returns an error with the reason.
func (s *Service) setApplicationInoperable(
	ctx context.Context,
	iApplicationAddress common.Address,
	id int64,
	reasonFmt string,
	args ...any,
) error {
	reason := fmt.Sprintf(reasonFmt, args...)
	appAddress := iApplicationAddress.String()

	// Log the reason first
	s.Logger.Error(reason, "application", appAddress)

	// Update application state
	err := s.repository.UpdateApplicationState(ctx, id, ApplicationState_Inoperable, &reason)
	if err != nil {
		s.Logger.Error("failed to update application state to inoperable", "app", appAddress, "err", err)
	}

	// Return the error with the reason
	return errors.New(reason)
}

func checkClaimConstraint(c *ClaimRow) error {
	zeroAddress := common.Address{}

	if c.FirstBlock > c.LastBlock {
		return ErrClaimMismatch
	}
	if c.IConsensusAddress == zeroAddress {
		return ErrClaimMismatch
	}
	if c.Status == EpochStatus_ClaimSubmitted {
		if c.ClaimHash == nil {
			return ErrClaimMismatch
		}
	}
	if c.Status == EpochStatus_ClaimAccepted || c.Status == EpochStatus_ClaimSubmitted {
		if c.ClaimTransactionHash == nil {
			return ErrClaimMismatch
		}
	}
	return nil
}

func checkClaimsConstraint(p *ClaimRow, c *ClaimRow) error {
	var err error

	err = checkClaimConstraint(c)
	if err != nil {
		return err
	}
	err = checkClaimConstraint(p)
	if err != nil {
		return err
	}

	// p, c consistent
	if p.IApplicationAddress != c.IApplicationAddress {
		return ErrClaimMismatch
	}
	if p.LastBlock > c.LastBlock {
		return ErrClaimMismatch
	}
	if p.FirstBlock > c.FirstBlock {
		return ErrClaimMismatch
	}
	if p.Index > c.Index {
		return ErrClaimMismatch
	}
	return nil
}

func claimSubmittedMatch(c *ClaimRow, e *iconsensus.IConsensusClaimSubmitted) bool {
	return c.IApplicationAddress == e.AppContract &&
		*c.ClaimHash == e.OutputsMerkleRoot &&
		c.LastBlock == e.LastProcessedBlockNumber.Uint64()
}

func claimAcceptedMatch(c *ClaimRow, e *iconsensus.IConsensusClaimAccepted) bool {
	return c.IApplicationAddress == e.AppContract &&
		*c.ClaimHash == e.OutputsMerkleRoot &&
		c.LastBlock == e.LastProcessedBlockNumber.Uint64()
}

func (s *Service) String() string {
	return s.Name
}
