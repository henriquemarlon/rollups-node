// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"log/slog"
	"time"

	"github.com/cartesi/rollups-node/internal/services/retry"
	"github.com/cartesi/rollups-node/pkg/contracts/iapplication"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

type ApplicationRetryPolicyDelegator struct {
	delegate          ApplicationContract
	maxRetries        uint64
	delayBetweenCalls time.Duration
	logger            *slog.Logger
}

func NewApplicationWithRetryPolicy(
	delegate ApplicationContract,
	maxRetries uint64,
	delayBetweenCalls time.Duration,
	logger *slog.Logger,
) *ApplicationRetryPolicyDelegator {
	return &ApplicationRetryPolicyDelegator{
		delegate:          delegate,
		maxRetries:        maxRetries,
		delayBetweenCalls: delayBetweenCalls,
		logger:            logger,
	}
}

func (d *ApplicationRetryPolicyDelegator) RetrieveOutputExecutionEvents(
	opts *bind.FilterOpts,
) ([]*iapplication.IApplicationOutputExecuted, error) {
	return retry.CallFunctionWithRetryPolicy(d.delegate.RetrieveOutputExecutionEvents,
		opts,
		d.logger,
		d.maxRetries,
		d.delayBetweenCalls,
		"Application::RetrieveOutputExecutionEvents",
	)
}
