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
}

type CreateInfo struct {
	service.CreateInfo

	Config config.Config

	Repository repository.Repository
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

	s.repository = c.Repository
	if s.repository == nil {
		return nil, fmt.Errorf("repository on validator service Create is nil")
	}

	return s, nil
}

func (s *Service) Alive() bool     { return true }
func (s *Service) Ready() bool     { return true }
func (s *Service) Reload() []error { return nil }
func (s *Service) Tick() []error {
	if err := s.Run(s.Context); err != nil {
		return []error{err}
	}
	return []error{}
}
func (s *Service) Stop(b bool) []error {
	return nil
}

func (v *Service) String() string {
	return v.Name
}

// The maximum height for the Merkle tree of all outputs produced
// by an application
const MAX_OUTPUT_TREE_HEIGHT = 63

type ValidatorRepository interface {
	ListApplications(ctx context.Context, f repository.ApplicationFilter, p repository.Pagination) ([]*Application, error)
	UpdateApplicationState(ctx context.Context, appID int64, state ApplicationState, reason *string) error

	ListOutputs(ctx context.Context, nameOrAddress string, f repository.OutputFilter, p repository.Pagination) ([]*Output, error)

	ListEpochs(ctx context.Context, nameOrAddress string, f repository.EpochFilter, p repository.Pagination) ([]*Epoch, error)

	GetLastInput(ctx context.Context, appAddress string, epochIndex uint64) (*Input, error) // FIXME migrate to list

	GetEpochByVirtualIndex(ctx context.Context, nameOrAddress string, index uint64) (*Epoch, error)

	StoreClaimAndProofs(ctx context.Context, epoch *Epoch, outputs []*Output) error
}

func getAllRunningApplications(ctx context.Context, er ValidatorRepository) ([]*Application, error) {
	f := repository.ApplicationFilter{State: Pointer(ApplicationState_Enabled)}
	return er.ListApplications(ctx, f, repository.Pagination{})
}

// Run executes the Validator main logic of producing claims and/or proofs
// for processed epochs of all running applications. It is meant to be executed
// inside a loop. If an error occurs while processing any epoch, it halts and
// returns the error.
func (v *Service) Run(ctx context.Context) error {
	apps, err := getAllRunningApplications(ctx, v.repository)
	if err != nil {
		return fmt.Errorf("failed to get running applications. %w", err)
	}

	for idx := range apps {
		if err := v.validateApplication(ctx, apps[idx]); err != nil {
			return err
		}
	}
	return nil
}

func getProcessedEpochs(ctx context.Context, er ValidatorRepository, address string) ([]*Epoch, error) {
	f := repository.EpochFilter{Status: Pointer(EpochStatus_InputsProcessed)}
	return er.ListEpochs(ctx, address, f, repository.Pagination{})
}

// validateApplication calculates, validates and stores the claim and/or proofs
// for each processed epoch of the application.
func (v *Service) validateApplication(ctx context.Context, app *Application) error {
	v.Logger.Debug("Starting validation", "application", app.Name)
	appAddress := app.IApplicationAddress.String()
	processedEpochs, err := getProcessedEpochs(ctx, v.repository, appAddress)
	if err != nil {
		return fmt.Errorf(
			"failed to get processed epochs of application %v. %w",
			app.IApplicationAddress, err,
		)
	}

	for _, epoch := range processedEpochs {
		v.Logger.Debug("Started calculating claim",
			"app", app.IApplicationAddress,
			"epoch_index", epoch.Index,
			"last_block", epoch.LastBlock,
		)
		claim, outputs, err := v.createClaimAndProofs(ctx, app, epoch)
		v.Logger.Info("Claim Computed",
			"app", app.IApplicationAddress,
			"epoch_index", epoch.Index,
			"last_block", epoch.LastBlock,
		)
		if err != nil {
			return err
		}

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
				"failed to get the machine claim for epoch %v of application %v. %w",
				epoch.Index, app.IApplicationAddress, err,
			)
		}

		if input.OutputsHash == nil {
			reason := fmt.Sprintf(
				"inconsistent state: machine claim for epoch %v of application %v was not found",
				epoch.Index, app.IApplicationAddress,
			)
			err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
			if err != nil {
				v.Logger.Error("failed to update application state to inoperable", "app", app.IApplicationAddress, "err", err)
			}
			return errors.New(reason)
		}

		// ...and compare it to the hash calculated by the Validator
		if *input.OutputsHash != *claim {
			reason := fmt.Sprintf(
				"validator claim does not match machine claim for epoch %v of application %v",
				epoch.Index, app.IApplicationAddress,
			)
			err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
			if err != nil {
				v.Logger.Error("failed to update application state to inoperable", "app", app.IApplicationAddress, "err", err)
			}
			return errors.New(reason)
		}

		// update the epoch status and its claim
		epoch.Status = EpochStatus_ClaimComputed
		epoch.ClaimHash = claim

		// store the epoch and proofs in the database
		err = v.repository.StoreClaimAndProofs(ctx, epoch, outputs)
		if err != nil {
			return fmt.Errorf(
				"failed to store claim and proofs for epoch %v of application %v. %w",
				epoch.Index, app.IApplicationAddress, err,
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

func getOutputsProducedInBlockRange(
	ctx context.Context,
	vr ValidatorRepository,
	address string,
	start uint64,
	end uint64,
) ([]*Output, error) {
	r := repository.Range{Start: start, End: end}
	f := repository.OutputFilter{BlockRange: Pointer(r)}
	return vr.ListOutputs(ctx, address, f, repository.Pagination{})
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
	epochOutputs, err := getOutputsProducedInBlockRange(
		ctx,
		v.repository,
		appAddress,
		epoch.FirstBlock,
		epoch.LastBlock,
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
			claim, _, err := merkle.CreateProofs(nil, MAX_OUTPUT_TREE_HEIGHT)
			if err != nil {
				reason := fmt.Sprintf(
					"failed to create proofs for epoch %v (%v) of application %v. %s",
					epoch.Index, epoch.VirtualIndex, appAddress, err.Error(),
				)
				err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
				if err != nil {
					v.Logger.Error("failed to update application state to inoperable", "app", appAddress, "err", err)
				}
				return nil, nil, errors.New(reason)
			}
			return &claim, nil, nil
		}
	} else {
		// if epoch has outputs, calculate a new claim and proofs
		var previousOutputs []*Output
		if previousEpoch != nil {
			// get all outputs created before the current epoch
			previousOutputs, err = getOutputsProducedInBlockRange(
				ctx,
				v.repository,
				appAddress,
				0, // Current implementation requires all outputs
				previousEpoch.LastBlock,
			)
			if err != nil {
				return nil, nil, fmt.Errorf(
					"failed to get all outputs of application %v before epoch %d. %w",
					appAddress, epoch.Index, err,
				)
			}
		}
		// the leaves of the Merkle tree are the Keccak256 hashes of all the
		// outputs
		leaves := make([]common.Hash, 0, len(epochOutputs)+len(previousOutputs))
		for idx := range previousOutputs {
			if previousOutputs[idx].Hash == nil {
				// should never happen
				reason := fmt.Sprintf(
					"missing hash of output %d from input %d",
					previousOutputs[idx].Index, previousOutputs[idx].InputIndex,
				)
				err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
				if err != nil {
					v.Logger.Error("failed to update application state to inoperable", "app", appAddress, "err", err)
				}
				return nil, nil, errors.New(reason)
			}
			leaves = append(leaves, *previousOutputs[idx].Hash)
		}
		for idx := range epochOutputs {
			hash := crypto.Keccak256Hash(epochOutputs[idx].RawData[:])
			// update outputs with their hash
			epochOutputs[idx].Hash = &hash
			// add them to the leaves slice
			leaves = append(leaves, hash)
		}

		claim, proofs, err := merkle.CreateProofs(leaves, MAX_OUTPUT_TREE_HEIGHT)
		if err != nil {
			reason := fmt.Sprintf(
				"failed to create proofs for epoch %v (%v) of application %v. %s",
				epoch.Index, epoch.VirtualIndex, appAddress, err.Error(),
			)
			err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
			if err != nil {
				v.Logger.Error("failed to update application state to inoperable", "app", appAddress, "err", err)
			}
			return nil, nil, errors.New(reason)
		}

		// update outputs with their proof
		for idx := range epochOutputs {
			start := epochOutputs[idx].Index * MAX_OUTPUT_TREE_HEIGHT
			end := (epochOutputs[idx].Index * MAX_OUTPUT_TREE_HEIGHT) + MAX_OUTPUT_TREE_HEIGHT
			epochOutputs[idx].OutputHashesSiblings = proofs[start:end]
		}
		return &claim, epochOutputs, nil
	}
	// if there are no outputs and there is a previous epoch, return its claim
	if previousEpoch.ClaimHash == nil {
		reason := fmt.Sprintf(
			"missing claim for previous epoch %v (current %v) of application %v. This should never happen",
			previousEpoch.Index, epoch.Index, appAddress,
		)
		err := v.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
		if err != nil {
			v.Logger.Error("failed to update application state to inoperable", "app", appAddress, "err", err)
		}
		return nil, nil, errors.New(reason)
	}
	return previousEpoch.ClaimHash, nil, nil
}
