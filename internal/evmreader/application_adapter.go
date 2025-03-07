// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"math/big"

	appcontract "github.com/cartesi/rollups-node/pkg/contracts/iapplication"
	"github.com/cartesi/rollups-node/pkg/ethutil"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// IConsensus Wrapper
type ApplicationContractAdapter struct {
	application        *appcontract.IApplication
	client             *ethclient.Client
	applicationAddress common.Address
}

func NewApplicationContractAdapter(
	appAddress common.Address,
	client *ethclient.Client,
) (*ApplicationContractAdapter, error) {
	applicationContract, err := appcontract.NewIApplication(appAddress, client)
	if err != nil {
		return nil, err
	}
	return &ApplicationContractAdapter{
		application:        applicationContract,
		applicationAddress: appAddress,
		client:             client,
	}, nil
}

func (a *ApplicationContractAdapter) GetConsensus(opts *bind.CallOpts) (common.Address, error) {
	return a.application.GetConsensus(opts)
}

func buildOutputExecutedFilterQuery(
	opts *bind.FilterOpts,
	applicationAddress common.Address,
) (q ethereum.FilterQuery, err error) {
	c, err := appcontract.IApplicationMetaData.GetAbi()
	if err != nil {
		return q, err
	}

	topics, err := abi.MakeTopics(
		[]interface{}{c.Events["OutputExecuted"].ID},
	)
	if err != nil {
		return q, err
	}

	q = ethereum.FilterQuery{
		Addresses: []common.Address{applicationAddress},
		FromBlock: new(big.Int).SetUint64(opts.Start),
		Topics:    topics,
	}
	if opts.End != nil {
		q.ToBlock = new(big.Int).SetUint64(*opts.End)
	}
	return q, err
}

func (a *ApplicationContractAdapter) RetrieveOutputExecutionEvents(
	opts *bind.FilterOpts,
) ([]*appcontract.IApplicationOutputExecuted, error) {
	q, err := buildOutputExecutedFilterQuery(opts, a.applicationAddress)
	if err != nil {
		return nil, err
	}

	itr, err := ethutil.ChunkedFilterLogs(opts.Context, a.client, q)
	if err != nil {
		return nil, err
	}

	var events []*appcontract.IApplicationOutputExecuted
	for log, err := range itr {
		if err != nil {
			return nil, err
		}
		ev, err := a.application.ParseOutputExecuted(*log)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}
