// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package ethutil

import (
	"context"
	"fmt"

	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/pkg/contracts/iapplicationfactory"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// NOTE: json formatting breaks if we don't use a separate type, don't know why
type ApplicationDeployment struct {
	model.Application `json:"application"`
	FactoryAddress    common.Address        `json:"factory"`
	OwnerAddress      common.Address        `json:"owner"`
	Salt              SaltBytes             `json:"salt"`
	WithAuthority     *AuthorityDeployment  `json:"authority,omitempty"`
	WithSelfHosted    *SelfhostedDeployment `json:"selfhosted,omitempty"`
}

// function does one of 3 things:
// - withSelfHosted != nil => application + authority (together via SelfHosted contract)
// - withAuthority != nil =>  application + authority (as two separate deployments)
// - withSelfHosted == nil && withAuthority == nil => application only
func (me *ApplicationDeployment) Deploy(ctx context.Context, client *ethclient.Client, txOpts *bind.TransactOpts) (common.Address, error) {
	var err error
	zero := common.Address{}
	var applicationAddress common.Address

	if me.WithSelfHosted != nil {
		applicationAddress, me.IConsensusAddress, err = me.WithSelfHosted.Deploy(ctx, client, txOpts)
		if err != nil {
			return zero, err
		}
		return applicationAddress, nil
	}

	if me.WithAuthority != nil {
		me.IConsensusAddress, err = me.WithAuthority.Deploy(ctx, client, txOpts)
		if err != nil {
			return zero, err
		}
	}

	factory, err := iapplicationfactory.NewIApplicationFactory(me.FactoryAddress, client)
	if err != nil {
		return zero, fmt.Errorf("failed to instantiate contract: %v", err)
	}

	tx, err := factory.NewApplication(txOpts, me.IConsensusAddress, me.OwnerAddress, me.TemplateHash, me.DataAvailability, me.Salt)
	if err != nil {
		return zero, fmt.Errorf("transaction failed: %v", err)
	}

	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return zero, fmt.Errorf("failed to wait for transaction mining: %v", err)
	}

	if receipt.Status != 1 {
		return zero, fmt.Errorf("transaction failed")
	}

	// Look for the specific event in the receipt logs
	for _, vLog := range receipt.Logs {
		// Parse log for ApplicationCreated event
		event, err := factory.ParseApplicationCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		return event.AppContract, nil
	}
	return zero, fmt.Errorf("failed to find ApplicationCreated event in receipt logs")
}
