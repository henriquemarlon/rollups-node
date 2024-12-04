// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"context"
	"log/slog"
	"time"

	"github.com/cartesi/rollups-node/internal/services/retry"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

type EthWsClientRetryPolicyDelegator struct {
	delegate          EthWsClient
	maxRetries        uint64
	delayBetweenCalls time.Duration
	logger            *slog.Logger
}

func NewEthWsClientWithRetryPolicy(
	delegate EthWsClient,
	maxRetries uint64,
	delayBetweenCalls time.Duration,
	logger *slog.Logger,
) *EthWsClientRetryPolicyDelegator {
	return &EthWsClientRetryPolicyDelegator{
		delegate:          delegate,
		maxRetries:        maxRetries,
		delayBetweenCalls: delayBetweenCalls,
		logger: logger,
	}
}

type subscribeNewHeadArgs struct {
	ctx context.Context
	ch  chan<- *types.Header
}

func (d *EthWsClientRetryPolicyDelegator) SubscribeNewHead(
	ctx context.Context,
	ch chan<- *types.Header,
) (ethereum.Subscription, error) {

	return retry.CallFunctionWithRetryPolicy(
		d.subscribeNewHead,
		subscribeNewHeadArgs{
			ctx: ctx,
			ch:  ch,
		},
		d.logger,
		d.maxRetries,
		d.delayBetweenCalls,
		"EthWSClient::SubscribeNewHead",
	)
}

func (d *EthWsClientRetryPolicyDelegator) subscribeNewHead(
	args subscribeNewHeadArgs,
) (ethereum.Subscription, error) {
	return d.delegate.SubscribeNewHead(args.ctx, args.ch)
}
