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
//     2. Some time after the submission, the computed claim shows up as a claimSubmission
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
	"fmt"
	"math/big"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"
	"github.com/cartesi/rollups-node/pkg/service"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	ErrClaimMismatch = fmt.Errorf("claim and antecessor mismatch")
	ErrEventMismatch = fmt.Errorf("Computed Claim mismatches ClaimSubmission event")
	ErrMissingEvent  = fmt.Errorf("accepted claim has no matching blockchain event")
)

type CreateInfo struct {
	service.CreateInfo

	Config config.Config

	EthConn    *ethclient.Client
	Repository repository.Repository
}

type Service struct {
	service.Service

	repository        repository.Repository
	ethConn           *ethclient.Client
	txOpts            *bind.TransactOpts
	claimsInFlight    map[common.Address]common.Hash // -> txHash
	submissionEnabled bool
	defaultBlock      config.DefaultBlock
}

const ClaimerConfigKey = "claimer"

type PersistentConfig struct {
	DefaultBlock           DefaultBlock
	ClaimSubmissionEnabled bool
	ChainID                uint64
}

func Create(ctx context.Context, c *CreateInfo) (*Service, error) {
	var err error
	if err = ctx.Err(); err != nil {
		return nil, err // This returns context.Canceled or context.DeadlineExceeded.
	}

	s := &Service{}
	c.CreateInfo.Impl = s

	err = service.Create(ctx, &c.CreateInfo, &s.Service)
	if err != nil {
		return nil, err
	}

	s.ethConn = c.EthConn
	if s.ethConn == nil {
		return nil, fmt.Errorf("ethclient on claimer service Create is nil")
	}

	chainId, err := s.ethConn.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	if chainId.Uint64() != c.Config.BlockchainId {
		return nil, fmt.Errorf("chainId mismatch: network %d != provided %d", chainId.Uint64(), c.Config.BlockchainId)
	}

	s.repository = c.Repository
	if s.repository == nil {
		return nil, fmt.Errorf("repository on claimer service Create is nil")
	}

	nodeConfig, err := s.setupPersistentConfig(ctx, &c.Config)
	if err != nil {
		return nil, err
	}
	if chainId.Uint64() != nodeConfig.ChainID {
		return nil, fmt.Errorf("NodeConfig chainId mismatch: network %d != config %d",
			chainId.Uint64(), nodeConfig.ChainID)
	}

	s.claimsInFlight = map[common.Address]common.Hash{}
	s.submissionEnabled = nodeConfig.ClaimSubmissionEnabled

	if s.submissionEnabled && s.txOpts == nil {
		s.txOpts, err = auth.GetTransactOpts(chainId)
		if err != nil {
			return nil, err
		}
	}
	s.defaultBlock = c.Config.BlockchainDefaultBlock

	return s, nil
}

func (s *Service) Alive() bool {
	return true
}

func (s *Service) Ready() bool {
	return true
}

func (s *Service) Reload() []error {
	return nil
}

func (s *Service) Stop(bool) []error {
	return nil
}

func (s *Service) Tick() []error {
	errs := []error{}
	endBlock, err := GetBlockNumber(s.Context, s.ethConn, s.defaultBlock)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	errs = append(errs, s.submitClaimsAndUpdateDatabase(s, endBlock)...)
	errs = append(errs, s.acceptClaimsAndUpdateDatabase(s, endBlock)...)

	return errs
}

/* transition claims from computed to submitted */
func (s *Service) submitClaimsAndUpdateDatabase(se sideEffects, endBlock *big.Int) []error {
	errs := []error{}
	acceptedOrSubmittedClaims, computedClaims, err := se.selectSubmissionClaimPairsPerApp()
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	// check claims in flight
	for key, txHash := range s.claimsInFlight {
		ready, receipt, err := se.pollTransaction(txHash, endBlock)
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
			err = se.updateEpochWithSubmittedClaim(claim, receipt.TxHash)
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
		var ic *iconsensus.IConsensus = nil
		var prevEvent *iconsensus.IConsensusClaimSubmission = nil
		var currEvent *iconsensus.IConsensusClaimSubmission = nil

		if _, isInFlight := s.claimsInFlight[key]; isInFlight {
			continue
		}

		prevClaimRow, prevExists := acceptedOrSubmittedClaims[key]
		if prevExists {
			err := checkClaimsConstraint(prevClaimRow, computedClaim)
			if err != nil {
				s.Logger.Error("database mismatch",
					"prevClaim", prevClaimRow,
					"currClaim", computedClaim,
					"err", err,
				)
				delete(computedClaims, key)
				errs = append(errs, err)
				// update application state to inoperable
				err = se.updateApplicationState(
					prevClaimRow.ApplicationID,
					ApplicationState_Inoperable,
					Pointer(err.Error()),
				)
				if err != nil {
					errs = append(errs, err)
				}
				goto nextApp
			}

			// if prevClaimRow exists, there must be a matching event
			ic, prevEvent, currEvent, err =
				se.findClaimSubmissionEventAndSucc(prevClaimRow, endBlock)
			if err != nil {
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			if prevEvent == nil {
				s.Logger.Error("missing event",
					"claim", prevClaimRow,
					"err", ErrMissingEvent,
				)
				delete(computedClaims, key)
				errs = append(errs, ErrMissingEvent)
				// update application state to inoperable
				err = se.updateApplicationState(
					prevClaimRow.ApplicationID,
					ApplicationState_Inoperable,
					Pointer(ErrMissingEvent.Error()),
				)
				if err != nil {
					errs = append(errs, err)
				}
				goto nextApp
			}
			if !claimSubmissionMatch(prevClaimRow, prevEvent) {
				s.Logger.Error("event mismatch",
					"claim", prevClaimRow,
					"event", prevEvent,
					"err", ErrEventMismatch,
				)
				delete(computedClaims, key)
				errs = append(errs, ErrEventMismatch)
				// update application state to inoperable
				err = se.updateApplicationState(
					prevClaimRow.ApplicationID,
					ApplicationState_Inoperable,
					Pointer(ErrEventMismatch.Error()),
				)
				if err != nil {
					errs = append(errs, err)
				}
				goto nextApp
			}
		} else {
			// first claim
			ic, currEvent, _, err =
				se.findClaimSubmissionEventAndSucc(computedClaim, endBlock)
			if err != nil {
				delete(computedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
		}

		if currEvent != nil {
			s.Logger.Debug("Found ClaimSubmitted Event",
				"app", currEvent.AppContract,
				"claim_hash", fmt.Sprintf("%x", currEvent.Claim),
				"last_block", currEvent.LastProcessedBlockNumber.Uint64(),
			)
			if !claimSubmissionMatch(computedClaim, currEvent) {
				s.Logger.Error("event mismatch",
					"claim", computedClaim,
					"event", currEvent,
					"err", ErrEventMismatch,
				)
				delete(computedClaims, key)
				errs = append(errs, ErrEventMismatch)
				// update application state to inoperable
				err = se.updateApplicationState(
					computedClaim.ApplicationID,
					ApplicationState_Inoperable,
					Pointer(ErrEventMismatch.Error()),
				)
				if err != nil {
					errs = append(errs, err)
				}
				goto nextApp
			}
			s.Logger.Debug("Updating claim status to submitted",
				"app", computedClaim.IApplicationAddress,
				"claim_hash", fmt.Sprintf("%x", computedClaim.ClaimHash),
				"last_block", computedClaim.LastBlock,
			)
			txHash := currEvent.Raw.TxHash
			err = se.updateEpochWithSubmittedClaim(computedClaim, txHash)
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
			txHash, err := se.submitClaimToBlockchain(ic, computedClaim)
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

func (s *Service) setupPersistentConfig(
	ctx context.Context,
	c *config.Config,
) (*PersistentConfig, error) {
	config, err := repository.LoadNodeConfig[PersistentConfig](ctx, s.repository, ClaimerConfigKey)
	if config == nil && err == nil {
		nc := NodeConfig[PersistentConfig]{
			Key: ClaimerConfigKey,
			Value: PersistentConfig{
				DefaultBlock:           c.BlockchainDefaultBlock,
				ClaimSubmissionEnabled: c.FeatureClaimSubmissionEnabled,
				ChainID:                c.BlockchainId,
			},
		}
		s.Logger.Info("Initializing claimer persistent config", "config", nc.Value)
		err = repository.SaveNodeConfig(ctx, s.repository, &nc)
		if err != nil {
			return nil, err
		}
		return &nc.Value, nil
	} else if err == nil {
		s.Logger.Info("Claimer was already configured. Using previous persistent config", "config", config.Value)
		return &config.Value, nil
	}

	s.Logger.Error("Could not retrieve persistent config from Database. %w", "error", err)
	return nil, err
}

/* transition claims from submitted to accepted */
func (s *Service) acceptClaimsAndUpdateDatabase(se sideEffects, endBlock *big.Int) []error {
	errs := []error{}
	acceptedClaims, submittedClaims, err := se.selectAcceptanceClaimPairsPerApp()
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	// check submitted claims
	for key, submittedClaim := range submittedClaims {
		var prevEvent *iconsensus.IConsensusClaimAcceptance = nil
		var currEvent *iconsensus.IConsensusClaimAcceptance = nil

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
				se.findClaimAcceptanceEventAndSucc(acceptedClaim, endBlock)
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
			if !claimAcceptanceMatch(acceptedClaim, prevEvent) {
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
				se.findClaimAcceptanceEventAndSucc(submittedClaim, endBlock)
			if err != nil {
				delete(submittedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
		}

		if currEvent != nil {
			s.Logger.Debug("Found ClaimAccepted Event",
				"app", currEvent.AppContract,
				"claim_hash", fmt.Sprintf("%x", currEvent.Claim),
				"last_block", currEvent.LastProcessedBlockNumber.Uint64(),
			)
			if !claimAcceptanceMatch(submittedClaim, currEvent) {
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
			err = se.updateEpochWithAcceptedClaim(submittedClaim, txHash)
			if err != nil {
				delete(submittedClaims, key)
				errs = append(errs, err)
				goto nextApp
			}
			s.Logger.Info("Claim accepted",
				"app", currEvent.AppContract,
				"event_block_number", currEvent.Raw.BlockNumber,
				"claim_hash", fmt.Sprintf("%x", currEvent.Claim),
				"last_block", currEvent.LastProcessedBlockNumber.Uint64(),
				"tx", txHash,
			)
		}
	nextApp:
	}
	return errs
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

func claimSubmissionMatch(c *ClaimRow, e *iconsensus.IConsensusClaimSubmission) bool {
	return c.IApplicationAddress == e.AppContract &&
		*c.ClaimHash == e.Claim &&
		c.LastBlock == e.LastProcessedBlockNumber.Uint64()
}

func claimAcceptanceMatch(c *ClaimRow, e *iconsensus.IConsensusClaimAcceptance) bool {
	return c.IApplicationAddress == e.AppContract &&
		*c.ClaimHash == e.Claim &&
		c.LastBlock == e.LastProcessedBlockNumber.Uint64()
}

func (s *Service) String() string {
	return s.Name
}
