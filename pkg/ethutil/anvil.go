// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package ethutil

import (
	"context"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func CreateAnvilSnapshotAndDeployApp(ctx context.Context, client *ethclient.Client, factoryAddr common.Address, templateHash common.Hash, dataAvailability []byte, salt string) (common.Address, func(), error) {
	var contractAddr common.Address
	if client == nil {
		return contractAddr, nil, fmt.Errorf("ethclient Client is nil")
	}

	// Create a snapshot of the current state
	snapshotID, err := CreateAnvilSnapshot(client.Client())
	if err != nil {
		return contractAddr, nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	chainId, err := client.ChainID(ctx)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return contractAddr, nil, fmt.Errorf("failed to retrieve chainID from Anvil: %w", err)
	}

	privateKey, err := MnemonicToPrivateKey(FoundryMnemonic, 0)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return contractAddr, nil, fmt.Errorf("failed to create privateKey: %w", err)
	}

	txOpts, err := bind.NewKeyedTransactorWithChainID(privateKey, chainId)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return contractAddr, nil, fmt.Errorf("failed to create TransactOpts: %w", err)
	}

	// Deploy the application contract
	contractAddr, err = DeploySelfHostedApplication(ctx, client, txOpts, factoryAddr, txOpts.From,
		templateHash, dataAvailability, salt)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return contractAddr, nil, fmt.Errorf("failed to deploy application contract: %w", err)
	}

	// Define a cleanup function to revert to the snapshot
	cleanup := func() {
		err := RevertToAnvilSnapshot(client.Client(), snapshotID)
		if err != nil {
			log.Printf("failed to revert to snapshot: %v", err)
		}
	}

	return contractAddr, cleanup, nil
}

func CreateAnvilSnapshot(rpcClient *rpc.Client) (string, error) {
	var snapshotID string
	// Using the JSON-RPC method "evm_snapshot" to create a snapshot
	err := rpcClient.Call(&snapshotID, "evm_snapshot")
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}
	return snapshotID, nil
}

func RevertToAnvilSnapshot(rpcClient *rpc.Client, snapshotID string) error {
	var success bool
	// Using the JSON-RPC method "evm_revert" to revert to the snapshot
	err := rpcClient.Call(&success, "evm_revert", snapshotID)
	if err != nil {
		return fmt.Errorf("failed to revert to snapshot: %w", err)
	}
	if !success {
		return fmt.Errorf("failed to revert to snapshot with ID: %s", snapshotID)
	}
	return nil
}

// Mines a new block
func MineNewBlock(
	ctx context.Context,
	client *ethclient.Client,
) (uint64, error) {
	if client == nil {
		return 0, fmt.Errorf("MineNewBlock: client is nil")
	}
	rpcClient := client.Client()
	err := rpcClient.CallContext(ctx, nil, "evm_mine")
	if err != nil {
		return 0, err
	}
	return client.BlockNumber(ctx)
}
