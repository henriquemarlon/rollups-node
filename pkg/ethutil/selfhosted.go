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
	FactoryAddress   common.Address `json:"selfhosted_factory"`
	OwnerAddress     common.Address `json:"owner"` // same for application and authority
	TemplateHash     common.Hash    `json:"template_hash"`
	DataAvailability []byte         `json:"data_availability"`
	EpochLength      uint64         `json:"epoch_length"`
	Salt             SaltBytes      `json:"salt"`

	// initialized during deploy
	ApplicationFactoryAddress common.Address `json:"application_factory"`
	AuthorityFactoryAddress   common.Address `json:"authority_factory"`
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

	// check if the factory address points to a deployed contract
	code, err := client.CodeAt(ctx, me.FactoryAddress, nil)
	if len(code) == 0 || err != nil {
		return zero, zero, fmt.Errorf("failed to verify the factory address during self hosted application deployment. No code at address")
	}

	receipt, err := sendTransaction(
		ctx, client, txOpts, big.NewInt(0), GasLimit,
		func(txOpts *bind.TransactOpts) (*types.Transaction, error) {
			me.ApplicationFactoryAddress, err = factory.GetApplicationFactory(nil)
			if err != nil {
				return nil, err
			}
			me.AuthorityFactoryAddress, err = factory.GetAuthorityFactory(nil)
			if err != nil {
				return nil, err
			}
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
	return zero, zero, fmt.Errorf("failed to obtain application address during self hosted application deployment. ApplicationCreated event not found in the recipe logs")

applicationEventFound:
	for _, vLog := range receipt.Logs {
		event, err := authorityFactory.ParseAuthorityCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		authorityAddress = event.Authority
		goto authorityEventFound
	}
	return zero, zero, fmt.Errorf("failed to obtain authority address during self hosted application deployment. AuthorityCreated event not found in the recipe logs")

authorityEventFound:
	return applicationAddress, authorityAddress, nil
}
