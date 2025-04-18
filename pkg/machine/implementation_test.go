// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package machine

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/pkg/emulator"
	"github.com/stretchr/testify/suite"
)

func TestImplementation(t *testing.T) {
	suite.Run(t, new(ImplementationSuite))
}

type ImplementationSuite struct {
	suite.Suite
	address string
	logger  *slog.Logger
}

func (s *ImplementationSuite) SetupSuite() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *ImplementationSuite) SetupTest() {
	_, address, _, err := StartServer(s.logger, FAST_DEADLINE)
	s.Require().Nil(err)
	s.address = address
}

func (s *ImplementationSuite) TearDownTest() {
	err := StopServer(s.address, s.logger, FAST_DEADLINE)
	s.Require().Nil(err)
}

// MockRemoteMachine mocks the emulator.RemoteMachine for testing
type MockRemoteMachine struct {
	LoadError           error
	RunReturn           emulator.BreakReason
	RunError            error
	ReadRegReturn       uint64
	ReadRegError        error
	GetRootHashReturn   []byte
	GetRootHashError    error
	ReadMemoryReturn    []byte
	ReadMemoryError     error
	WriteMemoryError    error
	WriteRegError       error
	StoreError          error
	ShutdownServerError error
	ForkServerReturn    *emulator.RemoteMachine
	ForkServerAddress   string
	ForkServerPid       uint32
	ForkServerError     error
}

// Test internal helper methods
func (s *ImplementationSuite) TestHelperMethods() {
	require := s.Require()
	ctx := context.Background()

	// Create a machine instance
	machine := &machineImpl{
		address: s.address,
		logger:  s.logger,
		params: model.ExecutionParameters{
			AdvanceIncCycles: 1000,
			AdvanceMaxCycles: 10000,
		},
	}

	// Test isAtManualYield with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := machine.isAtManualYield(canceledCtx)
	require.ErrorIs(err, ErrCanceled)

	// Test lastRequestWasAccepted with canceled context
	_, _, err = machine.wasLastRequestAccepted(canceledCtx)
	require.ErrorIs(err, ErrCanceled)

	// Test readCycle with canceled context
	_, err = machine.readMCycle(canceledCtx)
	require.ErrorIs(err, ErrCanceled)

}

// Test machine operations with mocked server
func (s *ImplementationSuite) TestMachineOperations() {
	require := s.Require()
	ctx := context.Background()

	// Create a machine instance
	machine := &machineImpl{
		address: s.address,
		logger:  s.logger,
		params: model.ExecutionParameters{
			AdvanceIncCycles: 1000,
			AdvanceMaxCycles: 10000,
		},
	}

	// Test Fork with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := machine.Fork(canceledCtx)
	require.ErrorIs(err, ErrCanceled)

	// Test Hash with canceled context
	_, err = machine.Hash(canceledCtx)
	require.ErrorIs(err, ErrCanceled)

	// Test OutputsHash with canceled context
	_, err = machine.OutputsHash(canceledCtx)
	require.ErrorIs(err, ErrCanceled)

	// Test Advance with canceled context
	_, _, _, _, err = machine.Advance(canceledCtx, []byte("test"))
	require.ErrorIs(err, ErrCanceled)

	// Test Inspect with canceled context
	_, _, err = machine.Inspect(canceledCtx, []byte("test"))
	require.ErrorIs(err, ErrCanceled)

	// Test Store with canceled context
	err = machine.Store(canceledCtx, "/tmp/test")
	require.ErrorIs(err, ErrCanceled)

	// Test Address
	address := machine.Address()
	require.Equal(s.address, address)
}

// Test process method
// func (s *ImplementationSuite) TestProcess() {
// 	require := s.Require()
// 	ctx := context.Background()
//
// 	// Create a machine instance
// 	machine := &machineImpl{
// 		address: s.address,
// 		logger:  s.logger,
// 		params: model.ExecutionParameters{
// 			AdvanceIncCycles: 1000,
// 			AdvanceMaxCycles: 10000,
// 		},
// 	}
//
// 	// Test with payload length exceeding limit
// 	// Create a large payload
// 	//largePayload := make([]byte, machine.backend.CmioRxBufferSize()+1)
// 	// _, _, _, _, err := machine.Advance(ctx, largePayload)
// 	// require.ErrorIs(err, ErrPayloadLengthLimitExceeded)
// }

// Test run method
func (s *ImplementationSuite) TestRun() {
	s.T().Skip("This test requires a properly initialized machine state")
}

// Test step method
func (s *ImplementationSuite) TestStep() {
	s.T().Skip("This test requires a properly initialized machine state")
}
