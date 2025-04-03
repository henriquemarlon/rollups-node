// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package advancer

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cartesi/rollups-node/internal/advancer/machines"
	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/inspect"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/service"
)

var (
	ErrInvalidMachines   = errors.New("machines must not be nil")
	ErrInvalidRepository = errors.New("repository must not be nil")

	ErrNoApp    = errors.New("no machine for application")
	ErrNoInputs = errors.New("no inputs")
)

type IAdvancerRepository interface {
	ListInputs(ctx context.Context, nameOrAddress string, f repository.InputFilter, p repository.Pagination) ([]*Input, uint64, error)
	StoreAdvanceResult(ctx context.Context, appID int64, ar *AdvanceResult) error
	UpdateEpochsInputsProcessed(ctx context.Context, nameOrAddress string) (int64, error)
	UpdateApplicationState(ctx context.Context, appID int64, state ApplicationState, reason *string) error
}

type IAdvancerMachines interface {
	GetAdvanceMachine(appId int64) (machines.AdvanceMachine, bool)
	UpdateMachines(ctx context.Context) error
	Apps() []*Application
}

type Service struct {
	service.Service
	repository     IAdvancerRepository
	machines       IAdvancerMachines
	inspector      *inspect.Inspector
	HTTPServer     *http.Server
	HTTPServerFunc func() error
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
		return nil, fmt.Errorf("repository on advancer service Create is nil")
	}

	machines := machines.New(ctx, c.Repository, c.Config.RemoteMachineLogLevel, s.Logger, c.Config.FeatureMachineHashCheckEnabled)
	s.machines = machines

	if c.Config.FeatureInspectEnabled {
		// allow partial construction for testing
		s.inspector, s.HTTPServer, s.HTTPServerFunc = inspect.NewInspector(
			c.Repository,
			machines,
			c.Config.InspectAddress,
			c.LogLevel,
			c.LogColor,
		)
	}

	return s, nil
}

func (s *Service) Alive() bool     { return true }
func (s *Service) Ready() bool     { return true }
func (s *Service) Reload() []error { return nil }
func (s *Service) Tick() []error {
	if err := s.Step(s.Context); err != nil {
		return []error{err}
	}
	return []error{}
}
func (s *Service) Stop(b bool) []error {
	return nil
}

func (s *Service) Serve() error {
	if s.inspector != nil && s.HTTPServerFunc != nil {
		go s.HTTPServerFunc()
	}
	return s.Service.Serve()
}

func (s *Service) String() string {
	return s.Name
}

func getUnprocessedInputs(ctx context.Context, mr IAdvancerRepository, appAddress string) ([]*Input, uint64, error) {
	f := repository.InputFilter{Status: Pointer(InputCompletionStatus_None)}
	return mr.ListInputs(ctx, appAddress, f, repository.Pagination{})
}

// Step steps the Advancer for one processing cycle.
// It gets unprocessed inputs from the repository,
// runs them through the cartesi machine,
// and updates the repository with the outputs.
func (advancer *Service) Step(ctx context.Context) error {
	// Dynamically updates the list of machines
	err := advancer.machines.UpdateMachines(ctx)
	if err != nil {
		return err
	}

	apps := advancer.machines.Apps()

	// Updates the status of the epochs.
	for _, app := range apps {
		// Gets the unprocessed inputs (of all apps) from the repository.
		advancer.Logger.Debug("Querying for unprocessed inputs")

		inputs, _, err := getUnprocessedInputs(ctx, advancer.repository, app.IApplicationAddress.String())
		if err != nil {
			return err
		}

		// Processes each set of inputs.
		advancer.Logger.Debug(fmt.Sprintf("Processing %d input(s) from %s", len(inputs), app.Name))
		err = advancer.process(ctx, app, inputs)
		if err != nil {
			return err
		}

		rows, err := advancer.repository.UpdateEpochsInputsProcessed(ctx, app.IApplicationAddress.String())
		if err != nil {
			return err
		}
		if rows > 0 {
			advancer.Logger.Info("Epochs updated to Inputs Processed", "application", app.Name, "count", rows)
		}
	}
	return nil
}

// process sequentially processes inputs from the the application.
func (advancer *Service) process(ctx context.Context, app *Application, inputs []*Input) error {
	// Asserts that the app has an associated machine.
	machine, exists := advancer.machines.GetAdvanceMachine(app.ID)
	if !exists {
		return fmt.Errorf("%w %d", ErrNoApp, app.ID)
	}

	if len(inputs) <= 0 {
		return nil
	}

	for _, input := range inputs {
		advancer.Logger.Info("Processing input", "application", app.Name, "index", input.Index)

		// Sends the input to the cartesi machine.
		res, err := machine.Advance(ctx, input.RawData, input.Index)
		if err != nil {
			advancer.Logger.Error("Error executing advance. Setting application to inoperable", "application", app.Name, "index", input.Index, "err", err)
			reason := err.Error()
			updateErr := advancer.repository.UpdateApplicationState(ctx, app.ID, ApplicationState_Inoperable, &reason)
			if updateErr != nil {
				advancer.Logger.Error("failed to update application state to inoperable", "application", app.Name, "err", updateErr)
			}
			return err
		}

		// Stores the result in the database.
		err = advancer.repository.StoreAdvanceResult(ctx, input.EpochApplicationID, res)
		if err != nil {
			return err
		}
	}

	return nil
}
