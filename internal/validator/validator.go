// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// Package validator provides components to create epoch claims and update
// rollups outputs with their proofs.
package validator

import (
	"context"
	"errors"
	"fmt"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/merkle"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Service struct {
	service.Service
	repository ValidatorRepository

	// cached constants
	pristineRootHash    common.Hash
	pristinePostContext []common.Hash
}

type CreateInfo struct {
	service.CreateInfo

	Config config.ValidatorConfig

	Repository repository.Repository
}

func Create(ctx context.Context, c *CreateInfo) (*Service, error) {
	var err error
	if err = ctx.Err(); err != nil {
		return nil, err // This returns context.Canceled or context.DeadlineExceeded.
	}

	s := &Service{}
	c.Impl = s

	err = service.Create(ctx, &c.CreateInfo, &s.Service)
	if err != nil {
		return nil, err
	}

	s.repository = c.Repository
	if s.repository == nil {
		return nil, fmt.Errorf("repository on validator service Create is nil")
	}

	s.pristinePostContext = merkle.CreatePostContext()
	s.pristineRootHash = s.pristinePostContext[merkle.TREE_DEPTH]

	return s, nil
}

func (s *Service) Alive() bool     { return true }
func (s *Service) Ready() bool     { return true }
func (s *Service) Reload() []error { return nil }

// Tick executes the Validator main logic of producing claims and/or proofs
// for processed epochs of all running applications.
func (s *Service) Tick() []error {
	apps, _, err := getAllRunningApplications(s.Context, s.repository)
	if err != nil {
		return []error{fmt.Errorf("failed to get running applications. %w", err)}
	}

	// validate each application
	errs := []error{}
	for idx := range apps {
		if err := s.validateApplication(s.Context, apps[idx]); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
func (s *Service) Stop(b bool) []error {
	return nil
}

func (v *Service) String() string {
	return v.Name
}

// The maximum height for the Merkle tree of all outputs produced
// by an application
const MAX_OUTPUT_TREE_HEIGHT = merkle.TREE_DEPTH

type ValidatorRepository interface {
	ListApplications(ctx context.Context, f repository.ApplicationFilter, p repository.Pagination) ([]*Application, uint64, error)
	UpdateApplicationState(ctx context.Context, appID int64, state ApplicationState, reason *string) error
	ListOutputs(ctx context.Context, nameOrAddress string, f repository.OutputFilter, p repository.Pagination) ([]*Output, uint64, error)
	GetLastOutputBeforeBlock(ctx context.Context, nameOrAddress string, block uint64) (*Output, error)
	ListEpochs(ctx context.Context, nameOrAddress string, f repository.EpochFilter, p repository.Pagination) ([]*Epoch, uint64, error)
	GetLastInput(ctx context.Context, appAddress string, epochIndex uint64) (*Input, error) // FIXME migrate to list
	GetEpochByVirtualIndex(ctx context.Context, nameOrAddress string, index uint64) (*Epoch, error)
	StoreClaimAndProofs(ctx context.Context, epoch *Epoch, outputs []*Output) error
}

func getAllRunningApplications(ctx context.Context, er ValidatorRepository) ([]*Application, uint64, error) {
	f := repository.ApplicationFilter{State: Pointer(ApplicationState_Enabled)}
	return er.ListApplications(ctx, f, repository.Pagination{})
}

func getProcessedEpochs(ctx context.Context, er ValidatorRepository, address string) ([]*Epoch, uint64, error) {
	f := repository.EpochFilter{Status: Pointer(EpochStatus_InputsProcessed)}
	return er.ListEpochs(ctx, address, f, repository.Pagination{})
}

// setApplicationInoperable marks an application as inoperable with the given reason,
// logs any error that occurs during the update, and returns an error with the reason.
func (v *Service) setApplicationInoperable(ctx context.Context, app *Application, reasonFmt string, args ...interface{}) error {
	reason := fmt.Sprintf(reasonFmt, args...)
	appAddress := app.IApplicationAddress.String()

	// Log the reason first
	v.Logger.Error(reason, "application", appAddress)

	// Update application state
	err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
	if err != nil {
		v.Logger.Error("failed to update application state to inoperable", "app", appAddress, "err", err)
	}

	// Return the error with the reason
	return errors.New(reason)
}

// validateApplication calculates, validates and stores the claim and/or proofs
// for each processed epoch of the application.
func (v *Service) validateApplication(ctx context.Context, app *Application) error {
	v.Logger.Debug("Starting validation", "application", app.Name)
	appAddress := app.IApplicationAddress.String()
	processedEpochs, _, err := getProcessedEpochs(ctx, v.repository, appAddress)
	if err != nil {
		return fmt.Errorf(
			"failed to get processed epochs of application %v. %w",
			app.IApplicationAddress, err,
		)
	}

	for _, epoch := range processedEpochs {
		v.Logger.Debug("Started calculating claim",
			"application", appAddress,
			"epoch_index", epoch.Index,
			"last_block", epoch.LastBlock,
		)
		claim, outputs, err := v.createClaimAndProofs(ctx, app, epoch)
		if err != nil {
			v.Logger.Error("failed to create claim and proofs.", "error", err)
			return err
		}

		v.Logger.Info("Claim Computed",
			"application", appAddress,
			"epoch_index", epoch.Index,
			"claimHash", *claim,
		)

		// The Cartesi Machine calculates the root hash of the outputs Merkle
		// tree after each input. Therefore, the root hash calculated after the
		// last input in the epoch must match the claim hash calculated by the
		// Validator. We first retrieve the hash calculated by the
		// Cartesi Machine...
		input, err := v.repository.GetLastInput(
			ctx,
			appAddress,
			epoch.Index,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to get the last Input for epoch %v of application %v. %w",
				epoch.Index, appAddress, err,
			)
		}

		if input.OutputsHash == nil {
			return v.setApplicationInoperable(ctx, app,
				"inconsistent state: machine claim for epoch %v of application %v was not found",
				epoch.Index, appAddress)
		}

		// ...and compare it to the hash calculated by the Validator
		if *input.OutputsHash != *claim {
			return v.setApplicationInoperable(ctx, app,
				"validator claim does not match machine claim for epoch %v of application %v. Expected: %v, Got %v",
				epoch.Index, appAddress, *input.OutputsHash, *claim)
		}

		// update the epoch status and its claim
		epoch.Status = EpochStatus_ClaimComputed
		epoch.ClaimHash = claim

		// store the epoch and proofs in the database
		err = v.repository.StoreClaimAndProofs(ctx, epoch, outputs)
		if err != nil {
			return fmt.Errorf(
				"failed to store claim and proofs for epoch %v of application %v. %w",
				epoch.Index, appAddress, err,
			)
		}
	}

	if len(processedEpochs) == 0 {
		v.Logger.Debug("no processed epochs to validate",
			"app", app.IApplicationAddress,
		)
	}

	return nil
}

// createClaimAndProofs calculates the claim and proofs for an epoch. It returns
// the claim and the epoch outputs updated with their hash and proofs. In case
// the epoch has no outputs, there are no proofs and it returns the pristine
// claim for the first epoch or the previous epoch claim otherwise.
func (v *Service) createClaimAndProofs(
	ctx context.Context,
	app *Application,
	epoch *Epoch,
) (*common.Hash, []*Output, error) {
	appAddress := app.IApplicationAddress.String()
	epochOutputs, _, err := v.repository.ListOutputs(ctx, appAddress, repository.OutputFilter{
		BlockRange: &repository.Range{
			Start: epoch.FirstBlock,
			End:   epoch.LastBlock,
		}},
		repository.Pagination{},
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get outputs for epoch %v (%v) of application %v. %w",
			epoch.Index, epoch.VirtualIndex, appAddress, err,
		)
	}

	var previousEpoch *Epoch
	if epoch.VirtualIndex > 0 {
		previousEpoch, err = v.repository.GetEpochByVirtualIndex(ctx, appAddress, epoch.VirtualIndex-1)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"failed to get previous epoch for epoch %v (%v) of application %v. %w",
				epoch.Index, epoch.VirtualIndex, appAddress, err,
			)
		}
	}

	// if there are no outputs
	if len(epochOutputs) == 0 {
		// and there is no previous epoch
		if previousEpoch == nil {
			// this is the first epoch, return the pristine claim
			return &v.pristineRootHash, nil, nil
		}
		// if there are no outputs and there is a previous epoch, return its claim
		if previousEpoch.ClaimHash == nil {
			return nil, nil, v.setApplicationInoperable(ctx, app,
				"invalid application state for epoch %v (%v) of application %v. Previous epoch has no claim.",
				epoch.Index, epoch.VirtualIndex, appAddress)
		}
		return previousEpoch.ClaimHash, nil, nil
	}

	var pre []common.Hash
	var index uint64
	// it there is no previous epoch
	if previousEpoch == nil {
		// there are only new outputs, use a dummy pre context
		pre = v.pristinePostContext
		index = 0
	} else {
		// retrieve the previous output, one not existing is ok... handled below
		lastOutput, err := v.repository.GetLastOutputBeforeBlock(ctx, appAddress, epoch.FirstBlock)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"failed to get previous output for epoch %v (%v) of application %v. %w",
				epoch.Index, epoch.VirtualIndex, appAddress, err,
			)
		}
		if lastOutput == nil {
			// there are only new outputs, use a dummy pre context
			pre = v.pristinePostContext
			index = 0
		} else {
			// there are previous outputs, create a pre context from the last output.
			if lastOutput.Hash == nil || len(lastOutput.OutputHashesSiblings) != merkle.TREE_DEPTH {
				return nil, nil, v.setApplicationInoperable(ctx, app,
					"Inconsistent application state (%v). Last output (%d) before epoch %d has no hash or invalid hash siblings.",
					app.Name, lastOutput.Index, epoch.Index)
			}
			pre = merkle.CreatePreContextFromProof(lastOutput.Index, *lastOutput.Hash, lastOutput.OutputHashesSiblings)
			index = lastOutput.Index + 1

			// make sure no output got skipped
			if index != epochOutputs[0].Index {
				return nil, nil, v.setApplicationInoperable(ctx, app,
					"Inconsistent application state (%v). Output index mismatch. "+
						"Last output (%d) before epoch %d and first output (%d) are not sequential.",
					app.Name, lastOutput.Index, epoch.Index, epochOutputs[0].Index)
			}
		}
	}

	// we have outputs to compute, gather the values to call ComputeSiblingsMatrix
	outputHashes := make([]common.Hash, 0, len(epochOutputs))
	for _, output := range epochOutputs {
		hash := crypto.Keccak256Hash(output.RawData[:])
		// update outputs with their hash
		output.Hash = &hash
		// add them to the leaves slice
		outputHashes = append(outputHashes, hash)
	}

	// compute and store siblings
	siblings, err := merkle.ComputeSiblingsMatrix(pre, outputHashes, v.pristinePostContext, index)
	if err != nil {
		return nil, nil, err
	}
	// update outputs with their siblings
	for idx, output := range epochOutputs {
		start := merkle.TREE_DEPTH * idx
		end := merkle.TREE_DEPTH * (idx + 1)
		output.OutputHashesSiblings = siblings[start:end]
	}
	rootHash := merkle.ComputeRootHashFromProof(epochOutputs[0].Index, *epochOutputs[0].Hash, epochOutputs[0].OutputHashesSiblings)
	return &rootHash, epochOutputs, nil
}
