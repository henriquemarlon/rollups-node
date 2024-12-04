// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"log/slog"
	"math/big"
	"time"

	"github.com/cartesi/rollups-node/internal/services/retry"
	"github.com/cartesi/rollups-node/pkg/contracts/iinputbox"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

type InputSourceWithRetryPolicyDelegator struct {
	delegate   InputSource
	maxRetries uint64
	delay      time.Duration
	logger     *slog.Logger
}

func NewInputSourceWithRetryPolicy(
	delegate InputSource,
	maxRetries uint64,
	delay time.Duration,
	logger     *slog.Logger,
) *InputSourceWithRetryPolicyDelegator {
	return &InputSourceWithRetryPolicyDelegator{
		delegate:   delegate,
		maxRetries: maxRetries,
		delay:      delay,
		logger:     logger,
	}
}

type retrieveInputsArgs struct {
	opts        *bind.FilterOpts
	appContract []common.Address
	index       []*big.Int
}

func (d *InputSourceWithRetryPolicyDelegator) RetrieveInputs(
	opts *bind.FilterOpts,
	appContract []common.Address,
	index []*big.Int,
) ([]iinputbox.IInputBoxInputAdded, error) {
	return retry.CallFunctionWithRetryPolicy(d.retrieveInputs,
		retrieveInputsArgs{
			opts:        opts,
			appContract: appContract,
			index:       index,
		},
		d.logger,
		d.maxRetries,
		d.delay,
		"InputSource::RetrieveInputs",
	)
}

func (d *InputSourceWithRetryPolicyDelegator) retrieveInputs(
	args retrieveInputsArgs,
) ([]iinputbox.IInputBoxInputAdded, error) {
	return d.delegate.RetrieveInputs(
		args.opts,
		args.appContract,
		args.index,
	)
}
