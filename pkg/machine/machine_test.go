// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package machine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestMachine(t *testing.T) {
	suite.Run(t, new(MachineSuite))
}

type MachineSuite struct {
	suite.Suite
	address string
	logger  *slog.Logger
}

func (s *MachineSuite) SetupSuite() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *MachineSuite) SetupTest() {
	_, address, _, err := StartServer(s.logger, FAST_DEADLINE)
	s.Require().Nil(err)
	s.address = address
}

func (s *MachineSuite) TearDownTest() {
	err := StopServer(s.address, s.logger, FAST_DEADLINE)
	s.Require().Nil(err)
}

func (s *MachineSuite) TestLoad() {
	require := s.Require()
	ctx := context.Background()

	// Test with nil logger
	machine, err := Load(ctx, nil, DefaultConfig("testdata/nonexistent"))
	require.Error(err)
	require.Nil(machine)
	require.Contains(err.Error(), "logger must not be nil")

	// Test with nil MachineConfig
	machine, err = Load(ctx, s.logger, nil)
	require.Error(err)
	require.Nil(machine)
	require.Contains(err.Error(), "MachineConfig must not be nil")

	// Test with invalid path
	machine, err = Load(ctx, s.logger, DefaultConfig("testdata/nonexistent"))
	require.Error(err)
	require.Nil(machine)
	require.ErrorIs(err, ErrMachineInternal)
	require.Contains(err.Error(), "could not load the machine")

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	machine, err = Load(canceledCtx, s.logger, DefaultConfig("testdata/nonexistent"))
	require.Error(err)
	require.Nil(machine)
	require.ErrorIs(err, ErrCanceled)

	// Test with timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	// Try to load a machine with a timed out context
	machine, err = Load(timeoutCtx, s.logger, DefaultConfig("testdata/nonexistent"))
	require.ErrorIs(err, ErrDeadlineExceeded)
	require.Nil(machine)
}

func (s *MachineSuite) TestMachineConfig() {
	require := s.Require()

	// Test default config
	config := DefaultConfig("some/path")
	require.NotNil(config)
	require.NotEmpty(config.Path)
	require.Nil(config.RuntimeConfig)
}

// MockMachine implements the Machine interface for testing
type MockMachine struct {
	ForkReturn Machine
	ForkError  error

	HashReturn Hash
	HashError  error

	OutputsHashReturn Hash
	OutputsHashError  error

	AdvanceAcceptedReturn bool
	AdvanceOutputsReturn  []Output
	AdvanceReportsReturn  []Report
	AdvanceHashReturn     Hash
	AdvanceError          error

	InspectAcceptedReturn bool
	InspectReportsReturn  []Report
	InspectError          error

	StoreError error

	CloseError error

	AddressReturn string
}

func (m *MockMachine) Fork(_ context.Context) (Machine, error) {
	return m.ForkReturn, m.ForkError
}

func (m *MockMachine) Hash(_ context.Context) (Hash, error) {
	return m.HashReturn, m.HashError
}

func (m *MockMachine) OutputsHash(_ context.Context) (Hash, error) {
	return m.OutputsHashReturn, m.OutputsHashError
}

func (m *MockMachine) Advance(_ context.Context, _ []byte) (
	bool, []Output, []Report, Hash, error,
) {
	return m.AdvanceAcceptedReturn,
		m.AdvanceOutputsReturn,
		m.AdvanceReportsReturn,
		m.AdvanceHashReturn,
		m.AdvanceError
}

func (m *MockMachine) Inspect(_ context.Context,
	_ []byte,
) (bool, []Report, error) {
	return m.InspectAcceptedReturn, m.InspectReportsReturn, m.InspectError
}

func (m *MockMachine) Store(_ context.Context, _ string) error {
	return m.StoreError
}

func (m *MockMachine) Close(_ context.Context) error {
	return m.CloseError
}

func (m *MockMachine) Address() string {
	return m.AddressReturn
}

// Test error handling
func (s *MachineSuite) TestErrors() {
	require := s.Require()

	// Test context errors
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	err := checkContext(ctx)
	require.ErrorIs(err, ErrDeadlineExceeded)

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	err = checkContext(ctx)
	require.ErrorIs(err, ErrCanceled)

	// Test error hierarchy
	var machineErr error = errors.New("test error")
	machineErr = errors.Join(ErrMachineInternal, machineErr)
	require.ErrorIs(machineErr, ErrMachineInternal)
}

// Test server management
func (s *MachineSuite) TestServer() {
	require := s.Require()

	// Test starting server with nil logger
	_, address, _, err := StartServer(nil, FAST_DEADLINE)
	require.Error(err)
	require.Empty(address)
	require.Contains(err.Error(), "logger must not be nil")

	// Test stopping server with nil logger
	err = StopServer("127.0.0.1:12345", nil, FAST_DEADLINE)
	require.Error(err)
	require.Contains(err.Error(), "logger must not be nil")

	// Test stopping non-existent server
	err = StopServer("127.0.0.1:12345", s.logger, FAST_DEADLINE)
	require.Error(err)
}
