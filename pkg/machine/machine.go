// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// Package machine provides a unified interface for interacting with Cartesi machines.
// It consolidates functionality from the previous rollupsmachine and cartesimachine packages.
package machine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cartesi/rollups-node/internal/model"
)

// Common type aliases
type (
	Cycle   = uint64
	Output  = []byte
	Report  = []byte
	Address = [20]byte
	Hash    = [32]byte
)

// Common errors
var (
	ErrMachineInternal            = errors.New("machine internal error")
	ErrDeadlineExceeded           = errors.New("machine operation deadline exceeded")
	ErrCanceled                   = errors.New("machine operation canceled")
	ErrOrphanServer               = errors.New("machine server was left orphan")
	ErrNotAtManualYield           = errors.New("not at manual yield")
	ErrException                  = errors.New("last request yielded an exception")
	ErrHalted                     = errors.New("machine halted")
	ErrOutputsLimitExceeded       = errors.New("outputs limit exceeded")
	ErrReachedTargetMcycle        = errors.New("machine reached target mcycle")
	ErrPayloadLengthLimitExceeded = errors.New("payload length limit exceeded")
	ErrHashLength                 = errors.New("hash does not have the exactly number of bytes")
)

// The Machine interface covers the core rollups-oriented functionalities of a cartesi
// machine: forking, getting the merkle tree's root hash, sending advance-state requests,
// sending inspect-state requests, and storing machine state.
type Machine interface {
	// Fork forks the machine.
	Fork(ctx context.Context) (Machine, error)
	// Hash returns the machine's merkle tree root hash.
	Hash(ctx context.Context) (Hash, error)
	// OutputsHash returns the outputs hash stored in the cmio tx buffer.
	OutputsHash(ctx context.Context) (Hash, error)

	// Advance sends an input to the machine.
	// It returns a boolean indicating whether or not the request was accepted.
	// It also returns the corresponding outputs, reports, and the hash of the outputs.
	// In case the request is not accepted, the function does not return outputs.
	Advance(ctx context.Context, input []byte) (bool, []Output, []Report, Hash, error)

	// Inspect sends a query to the machine.
	// It returns a boolean indicating whether or not the request was accepted
	// It also returns the corresponding reports.
	Inspect(ctx context.Context, query []byte) (bool, []Report, error)

	// Store saves the machine state to the specified path.
	Store(ctx context.Context, path string) error

	// Close closes the inner cartesi machine.
	// It returns nil if the machine has already been closed.
	Close() error

	// Server information
	Address() string
}

// MachineConfig contains configuration for a machine instance
type MachineConfig struct {
	Address             string                    // Address to connect to the machine backend
	Path                string                    // Path to the machine's directory
	ExecutionParameters model.ExecutionParameters // Execution parameters for the machine
	RuntimeConfig       *string
	BackendFactoryFn    BackendFactory // Optional factory for custom backend creation
}

// DefaultConfig returns a default machine configuration
func DefaultConfig(path string) *MachineConfig {
	return &MachineConfig{
		Address: "127.0.0.1:0",
		Path:    path,
		ExecutionParameters: model.ExecutionParameters{
			AdvanceIncCycles:   4194304,             // nolint: mnd
			AdvanceMaxCycles:   4611686018427387903, // nolint: mnd
			InspectIncCycles:   4194304,             // nolint: mnd
			InspectMaxCycles:   4611686018427387903, // nolint: mnd
			AdvanceIncDeadline: time.Second * 10,    // nolint: mnd
			AdvanceMaxDeadline: time.Second * 180,   // nolint: mnd
			InspectIncDeadline: time.Second * 10,    // nolint: mnd
			InspectMaxDeadline: time.Second * 180,   // nolint: mnd
			LoadDeadline:       time.Second * 300,   // nolint: mnd
			StoreDeadline:      time.Second * 180,   // nolint: mnd
			FastDeadline:       time.Second * 5,     // nolint: mnd
		},
		BackendFactoryFn: DefaultBackendFactory, // Use the default backend factory
	}
}

func DefaultBackendFactory(address string, timeout time.Duration) (Backend, string, uint32, error) {
	return NewLibCartesiBackend(address, timeout)
}

// Load loads a machine from a snapshot or template path
func Load(ctx context.Context, logger *slog.Logger, config *MachineConfig) (Machine, error) {
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	if config == nil {
		return nil, errors.New("MachineConfig must not be nil")
	}

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	// Use the backend factory from config, or default to emulator.SpawnServer
	backendFactory := config.BackendFactoryFn
	if backendFactory == nil {
		backendFactory = DefaultBackendFactory
	}

	backend, address, pid, err := backendFactory(config.Address, config.ExecutionParameters.FastDeadline)
	if err != nil {
		return nil, errors.Join(ErrMachineInternal, err)
	}

	if config.RuntimeConfig == nil {
		runtimeConf, err := backend.NewMachineRuntimeConfig()
		if err != nil {
			shutdownErr := backend.ShutdownServer(config.ExecutionParameters.FastDeadline)
			backend.Delete()
			err = fmt.Errorf("could not instantiate new machine runtime config: %w", err)
			return nil, errors.Join(ErrMachineInternal, err, shutdownErr)
		}
		config.RuntimeConfig = &runtimeConf
	}

	machine := &machineImpl{
		backend: backend,
		address: address,
		pid:     pid,
		logger:  logger,
		params:  config.ExecutionParameters,
	}

	if err := checkContext(ctx); err != nil {
		machine.Close()
		return nil, err
	}

	// Loads the machine stored at path into the server.
	err = machine.backend.Load(config.Path, *config.RuntimeConfig, machine.params.LoadDeadline)
	if err != nil {
		machine.Close()
		err = fmt.Errorf("could not load the machine: %w", err)
		return nil, errors.Join(ErrMachineInternal, err)
	}

	// Ensures that the machine is at a manual yield.
	isAtManualYield, err := machine.isAtManualYield(ctx)
	if err != nil {
		machine.Close()
		return nil, err
	}
	if !isAtManualYield {
		machine.Close()
		return nil, ErrNotAtManualYield
	}

	// Ensures that the last request the machine received did not yield an exception.
	_, _, err = machine.wasLastRequestAccepted(ctx)
	if err != nil {
		machine.Close()
		return nil, err
	}

	return machine, nil
}
