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
	maxRetries      uint64
	maxDelay        time.Duration
	ethClient       *ethclient.Client
	iConsensusCache map[common.Address]ConsensusContract
}

func NewEvmReaderContractFactory(
	ethClient *ethclient.Client,
	maxRetries uint64,
	maxDelay time.Duration,

) *EvmReaderContractFactory {
	return &EvmReaderContractFactory{
		ethClient:       ethClient,
		maxRetries:      maxRetries,
		maxDelay:        maxDelay,
		iConsensusCache: make(map[common.Address]ConsensusContract),
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

func (f *EvmReaderContractFactory) NewIConsensus(
	address common.Address,
) (ConsensusContract, error) {

	logger := service.NewLogger(slog.LevelDebug, true)
	delegator, ok := f.iConsensusCache[address]
	if !ok {
		// Building a contract does not fail due to network errors.
		// No need to retry this operation
		consensus, err := NewConsensusContractAdapter(address, f.ethClient)
		if err != nil {
			return nil, err
		}

		delegator = NewConsensusWithRetryPolicy(consensus, f.maxRetries, f.maxDelay, logger)

		f.iConsensusCache[address] = delegator
	}
	return delegator, nil

}
