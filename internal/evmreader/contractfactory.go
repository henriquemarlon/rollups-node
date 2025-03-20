// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package evmreader

import (
	"log/slog"
	"time"

	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Builds contracts delegates that will
// use retry policy on contract methods calls
type EvmReaderContractFactory struct {
	maxRetries uint64
	maxDelay   time.Duration
	ethClient  *ethclient.Client
}

func NewEvmReaderContractFactory(
	ethClient *ethclient.Client,
	maxRetries uint64,
	maxDelay time.Duration,
) *EvmReaderContractFactory {
	return &EvmReaderContractFactory{
		ethClient:  ethClient,
		maxRetries: maxRetries,
		maxDelay:   maxDelay,
	}
}

func (f *EvmReaderContractFactory) NewApplication(
	address common.Address,
) (ApplicationContract, error) {

	// Building a contract does not fail due to network errors.
	// No need to retry this operation
	applicationContract, err := NewApplicationContractAdapter(address, f.ethClient)
	if err != nil {
		return nil, err
	}

	logger := service.NewLogger(slog.LevelDebug, true)
	return NewApplicationWithRetryPolicy(applicationContract, f.maxRetries, f.maxDelay, logger), nil

}

func (f *EvmReaderContractFactory) NewInputSource(
	address common.Address,
) (InputSource, error) {

	// Building a contract does not fail due to network errors.
	// No need to retry this operation
	inputSourceContract, err := NewInputSourceAdapter(address, f.ethClient)
	if err != nil {
		return nil, err
	}

	logger := service.NewLogger(slog.LevelDebug, true)
	return NewInputSourceWithRetryPolicy(inputSourceContract, f.maxRetries, f.maxDelay, logger), nil

}
