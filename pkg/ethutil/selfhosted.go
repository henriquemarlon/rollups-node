// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package ethutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cartesi/rollups-node/pkg/contracts/iapplicationfactory"
	"github.com/cartesi/rollups-node/pkg/contracts/iauthorityfactory"
	"github.com/cartesi/rollups-node/pkg/contracts/iselfhostedapplicationfactory"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type SelfhostedDeployment struct {
	FactoryAddress            common.Address `json:"factory"`
	ApplicationFactoryAddress common.Address `json:"application"`
	AuthorityFactoryAddress   common.Address `json:"authority"`
	OwnerAddress              common.Address `json:"owner"` // same for application and authority
	TemplateHash              common.Hash    `json:"template_hash"`
	DataAvailability          []byte         `json:"data_availability"`
	EpochLength               uint64         `json:"epoch_length"`
	Salt                      SaltBytes      `json:"salt"`
}

func (me *SelfhostedDeployment) Deploy(
	ctx context.Context,
	client *ethclient.Client,
	txOpts *bind.TransactOpts,
) (common.Address, common.Address, error) {
	zero := common.Address{}
	factory, err := iselfhostedapplicationfactory.NewISelfHostedApplicationFactory(me.FactoryAddress, client)
	if err != nil {
		return zero, zero, err
	}

	receipt, err := sendTransaction(
		ctx, client, txOpts, big.NewInt(0), GasLimit,
		func(txOpts *bind.TransactOpts) (*types.Transaction, error) {
			return factory.DeployContracts(txOpts, me.OwnerAddress, big.NewInt(0).SetUint64(me.EpochLength), me.OwnerAddress, me.TemplateHash,
				me.DataAvailability, me.Salt)
		},
	)
	if err != nil {
		return zero, zero, err
	}

	applicationFactory, err := iapplicationfactory.NewIApplicationFactory(me.ApplicationFactoryAddress, client)
	if err != nil {
		return zero, zero, err
	}

	authorityFactory, err := iauthorityfactory.NewIAuthorityFactory(me.AuthorityFactoryAddress, client)
	if err != nil {
		return zero, zero, err
	}

	var applicationAddress common.Address
	var authorityAddress common.Address
	for _, vLog := range receipt.Logs {
		event, err := applicationFactory.ParseApplicationCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		applicationAddress = event.AppContract
		goto applicationEventFound
	}
	// searched logs, didn't find event
	return zero, zero, fmt.Errorf("failed to obtain application address of self hosted application deployment")

applicationEventFound:
	for _, vLog := range receipt.Logs {
		event, err := authorityFactory.ParseAuthorityCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		authorityAddress = event.Authority
		goto authorityEventFound
	}
	// searched logs, didn't find event
	return zero, zero, fmt.Errorf("failed to obtain authority address of self hosted application deployment")

authorityEventFound:
	return applicationAddress, authorityAddress, nil
}
