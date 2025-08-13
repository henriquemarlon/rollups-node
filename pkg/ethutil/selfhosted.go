// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package ethutil

import (
	"context"
	"encoding/hex"
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

type SelfhostedApplicationDeployment struct {
	FactoryAddress          common.Address `json:"factory_address"`
	ApplicationOwnerAddress common.Address `json:"application_owner"`
	AuthorityOwnerAddress   common.Address `json:"authority_owner"`
	TemplateHash            common.Hash    `json:"template_hash"`
	DataAvailability        []byte         `json:"-"`
	EpochLength             uint64         `json:"epoch_length"`
	Salt                    SaltBytes      `json:"salt"`

	InputBoxAddress common.Address `json:"inputbox_address"`
	IInputBoxBlock  uint64         `json:"inputbox_block"`

	Verbose bool
}

type SelfhostedApplicationDeploymentResult struct {
	Deployment *SelfhostedApplicationDeployment `json:"deployment"`

	ApplicationAddress common.Address `json:"application_address"`
	AuthorityAddress   common.Address `json:"authority_address"`

	AuthorityFactoryAddress   common.Address `json:"authority_factory"`
	ApplicationFactoryAddress common.Address `json:"application_factory"`
}

func (me *SelfhostedApplicationDeployment) String() string {
	result := ""
	result += fmt.Sprintf("selfhosted deployment:\n")
	result += fmt.Sprintf("\tapplication owner:     %v\n", me.ApplicationOwnerAddress)
	result += fmt.Sprintf("\tauthority owner:       %v\n", me.AuthorityOwnerAddress)
	if me.Verbose {
		result += fmt.Sprintf("\tfactory address:       %v\n", me.FactoryAddress)
		result += fmt.Sprintf("\ttemplate hash:         %v\n", me.TemplateHash)
		result += fmt.Sprintf("\tdata availability:     0x%v\n", hex.EncodeToString(me.DataAvailability))
		result += fmt.Sprintf("\tsalt:                  %v\n", me.Salt)
		result += fmt.Sprintf("\tepoch length:          %v\n", me.EpochLength)
	}
	return result
}

func (me *SelfhostedApplicationDeploymentResult) String() string {
	result := ""
	result += fmt.Sprintf("\tapplication address:   %v\n", me.ApplicationAddress)
	result += fmt.Sprintf("\tauthority address:     %v\n", me.AuthorityAddress)
	if me.Deployment.Verbose {
		result += fmt.Sprintf("\tapplication factory:   %v\n", me.ApplicationFactoryAddress)
		result += fmt.Sprintf("\tauthority factory:     %v\n", me.AuthorityFactoryAddress)
	}
	return result
}

func (me *SelfhostedApplicationDeployment) Deploy(
	ctx context.Context,
	client *ethclient.Client,
	txOpts *bind.TransactOpts,
) (common.Address, IApplicationDeploymentResult, error) {
	zero := common.Address{}
	result := &SelfhostedApplicationDeploymentResult{}
	result.Deployment = me
	factory, err := iselfhostedapplicationfactory.NewISelfHostedApplicationFactory(me.FactoryAddress, client)
	if err != nil {
		return zero, nil, err
	}

	receipt, err := sendTransaction(
		ctx, client, txOpts, big.NewInt(0), GasLimit,
		func(txOpts *bind.TransactOpts) (*types.Transaction, error) {
			result.ApplicationFactoryAddress, err = factory.GetApplicationFactory(nil)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve application factory address: %w", err)
			}
			result.AuthorityFactoryAddress, err = factory.GetAuthorityFactory(nil)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve authority factory address: %w", err)
			}
			return factory.DeployContracts(txOpts, me.AuthorityOwnerAddress, big.NewInt(0).SetUint64(me.EpochLength), me.ApplicationOwnerAddress,
				me.TemplateHash, me.DataAvailability, me.Salt)
		},
	)
	if err != nil {
		return zero, nil, fmt.Errorf("failed to create a self hosted application: execution reverted")
	}

	applicationFactory, err := iapplicationfactory.NewIApplicationFactory(result.ApplicationFactoryAddress, client)
	if err != nil {
		return zero, nil, err
	}

	authorityFactory, err := iauthorityfactory.NewIAuthorityFactory(result.AuthorityFactoryAddress, client)
	if err != nil {
		return zero, nil, err
	}

	for _, vLog := range receipt.Logs {
		event, err := applicationFactory.ParseApplicationCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		result.ApplicationAddress = event.AppContract
		goto applicationEventFound
	}
	return zero, nil, fmt.Errorf("failed to obtain application address during self hosted application deployment. ApplicationCreated event not found in the receipt logs")

applicationEventFound:
	for _, vLog := range receipt.Logs {
		event, err := authorityFactory.ParseAuthorityCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		result.AuthorityAddress = event.Authority
		goto authorityEventFound
	}
	return zero, nil, fmt.Errorf("failed to obtain authority address during self hosted application deployment. AuthorityCreated event not found in the recipe logs")

authorityEventFound:
	return result.ApplicationAddress, result, nil
}

func (me *SelfhostedApplicationDeployment) GetFactoryAddress() common.Address {
	return me.FactoryAddress
}
