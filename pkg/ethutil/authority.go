// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package ethutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cartesi/rollups-node/pkg/contracts/iauthorityfactory"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type AuthorityDeployment struct {
	Address        common.Address `json:"address"`
	FactoryAddress common.Address `json:"factory"`
	OwnerAddress   common.Address `json:"owner"`
	EpochLength    uint64         `json:"epoch_length"`
	Salt           SaltBytes      `json:"salt"`
}

func (me AuthorityDeployment) Deploy(ctx context.Context, client *ethclient.Client, txOpts *bind.TransactOpts) (common.Address, error) {
	contract, err := iauthorityfactory.NewIAuthorityFactory(me.FactoryAddress, client)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to instantiate contract: %v", err)
	}

	tx, err := contract.NewAuthority0(txOpts, me.OwnerAddress, big.NewInt(int64(me.EpochLength)), me.Salt)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to create new authority: %v", err)
	}

	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to mine new authority transaction: %v", err)
	}

	if receipt.Status != 1 {
		return common.Address{}, fmt.Errorf("transaction failed")
	}

	// search for the matching event
	for _, vLog := range receipt.Logs {
		event, err := contract.ParseAuthorityCreated(*vLog)
		if err != nil {
			continue // Skip logs that don't match
		}
		return event.Authority, nil
	}
	return common.Address{}, fmt.Errorf("failed to find event in receipt logs")
}
