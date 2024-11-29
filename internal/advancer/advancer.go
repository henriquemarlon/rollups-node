// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package advancer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cartesi/rollups-node/internal/advancer/machines"
	"github.com/cartesi/rollups-node/internal/config"

	"github.com/cartesi/rollups-node/internal/inspect"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/nodemachine"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/rollupsmachine/cartesimachine"
	"github.com/cartesi/rollups-node/pkg/service"
)

var (
	ErrInvalidMachines   = errors.New("machines must not be nil")
	ErrInvalidRepository = errors.New("repository must not be nil")

	ErrNoApp    = errors.New("no machine for application")
	ErrNoInputs = errors.New("no inputs")
)

type IAdvancerRepository interface {
	// Only needs Id, Index, and RawData fields from the retrieved Inputs.
	GetUnprocessedInputs(_ context.Context, apps []Address) (map[Address][]*Input, error)
	StoreAdvanceResult(context.Context, *Input, *nodemachine.AdvanceResult) error
	UpdateClosedEpochs(_ context.Context, app Address) error
}

type IAdvancerMachines interface {
	GetAdvanceMachine(app Address) (machines.AdvanceMachine, bool)
	UpdateMachines(ctx context.Context) error
	Apps() []Address
}

type Advancer struct {
	repository IAdvancerRepository
	machines   IAdvancerMachines
}

type Service struct {
	service.Service
	Advancer
	inspector  *inspect.Inspector
}

func New(machines IAdvancerMachines, repository IAdvancerRepository) (*Advancer, error) {
	if machines == nil {
		return nil, ErrInvalidMachines
	}
	if repository == nil {
		return nil, ErrInvalidRepository
	}
	return &Advancer{machines: machines, repository: repository}, nil
}

type CreateInfo struct {
	service.CreateInfo
	AdvancerPollingInterval time.Duration
	PostgresEndpoint        config.Redacted[string]
	PostgresSslMode         bool
	Repository              *repository.Database
	HttpAddress             string
	HttpPort                int
	MachineServerVerbosity  config.Redacted[cartesimachine.ServerVerbosity]
	Machines                *machines.Machines
}

func (c *CreateInfo) LoadEnv() {
	c.PostgresEndpoint.Value = config.GetPostgresEndpoint()
	c.PollInterval = config.GetAdvancerPollingInterval()
	c.HttpAddress = config.GetHttpAddress()
	c.HttpPort = config.GetHttpPort()
	c.MachineServerVerbosity.Value =
		cartesimachine.ServerVerbosity(config.GetMachineServerVerbosity())
	c.LogLevel = service.LogLevel(config.GetLogLevel())
}

func Create(c *CreateInfo, s *Service) error {
	err := service.Create(&c.CreateInfo, &s.Service)
	if err != nil {
		return err
	}

	if c.Repository == nil {
		c.Repository, err = repository.Connect(s.Context, c.PostgresEndpoint.Value)
		if err != nil {
			return err
		}
	}
	s.repository = c.Repository

	if c.Machines == nil {
		c.Machines, err = machines.Load(s.Context, c.Repository, c.MachineServerVerbosity.Value)
		if err != nil {
			return err
		}
	}
	s.machines = c.Machines

	if s.Service.ServeMux == nil {
		if c.CreateInfo.ServeMux == nil {
			c.ServeMux = http.NewServeMux()
		}
	}
	s.Service.ServeMux.Handle("/inspect/{dapp}", http.Handler(s.inspector))
	s.Service.ServeMux.Handle("/inspect/{dapp}/{payload}", http.Handler(s.inspector))

	return nil
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

func (s *Service) Start(context context.Context, ready chan<- struct{}) error {
	ready <- struct{}{}
	return s.Serve()
}
func (v *Service) String() string {
	return v.Name
}

// Step steps the Advancer for one processing cycle.
// It gets unprocessed inputs from the repository,
// runs them through the cartesi machine,
// and updates the repository with the outputs.
func (advancer *Advancer) Step(ctx context.Context) error {
	// Dynamically updates the list of machines
	err := advancer.machines.UpdateMachines(ctx)
	if err != nil {
		return err
	}

	apps := advancer.machines.Apps()

	// Gets the unprocessed inputs (of all apps) from the repository.
	slog.Debug("advancer: querying for unprocessed inputs")
	inputs, err := advancer.repository.GetUnprocessedInputs(ctx, apps)
	if err != nil {
		return err
	}

	// Processes each set of inputs.
	for app, inputs := range inputs {
		slog.Debug(fmt.Sprintf("advancer: processing %d input(s) from %v", len(inputs), app))
		err := advancer.process(ctx, app, inputs)
		if err != nil {
			return err
		}
	}

	// Updates the status of the epochs.
	for _, app := range apps {
		err := advancer.repository.UpdateClosedEpochs(ctx, app)
		if err != nil {
			return err
		}
	}

	return nil
}

// process sequentially processes inputs from the the application.
func (advancer *Advancer) process(ctx context.Context, app Address, inputs []*Input) error {
	// Asserts that the app has an associated machine.
	machine, exists := advancer.machines.GetAdvanceMachine(app)
	if !exists {
		panic(fmt.Errorf("%w %s", ErrNoApp, app.String()))
	}

	// Asserts that there are inputs to process.
	if len(inputs) <= 0 {
		panic(ErrNoInputs)
	}

	// FIXME if theres a change in epoch id call update epochs
	for _, input := range inputs {
		slog.Info("advancer: Processing input", "app", app, "id", input.Id, "index", input.Index)

		// Sends the input to the cartesi machine.
		res, err := machine.Advance(ctx, input.RawData, input.Index)
		if err != nil {
			return err
		}

		// Stores the result in the database.
		err = advancer.repository.StoreAdvanceResult(ctx, input, res)
		if err != nil {
			return err
		}
	}

	return nil
}

