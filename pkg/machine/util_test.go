// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package machine

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const FAST_DEADLINE = 3 * time.Second

func TestServer(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

type ServerSuite struct {
	suite.Suite
	logger *slog.Logger
}

func (s *ServerSuite) SetupSuite() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *ServerSuite) TestStopServer() {
	require := s.Require()

	// Test with nil logger
	err := StopServer("127.0.0.1:12345", nil, FAST_DEADLINE)
	require.Error(err)
	require.Contains(err.Error(), "logger must not be nil")

	// Test with invalid address
	err = StopServer("invalid:address", s.logger, FAST_DEADLINE)
	require.Error(err)

	// Test with non-existent server
	err = StopServer("127.0.0.1:54321", s.logger, FAST_DEADLINE)
	require.Error(err)
}
