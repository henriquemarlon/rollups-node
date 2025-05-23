// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package ethutil

import (
	"context"
	"fmt"
	"log"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/pkg/contracts/iselfhostedapplicationfactory"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func CreateAnvilSnapshotAndDeployApp(ctx context.Context, client *ethclient.Client, factoryAddr common.Address, templateHash common.Hash, dataAvailability []byte, salt string) (common.Address, func(), error) {
	zero := common.Address{}
	if client == nil {
		return zero, nil, fmt.Errorf("ethclient Client is nil")
	}

	// Create a snapshot of the current state
	snapshotID, err := CreateAnvilSnapshot(client.Client())
	if err != nil {
		return zero, nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	chainId, err := client.ChainID(ctx)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return zero, nil, fmt.Errorf("failed to retrieve chainID from Anvil: %w", err)
	}

	privateKey, err := MnemonicToPrivateKey(FoundryMnemonic, 0)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return zero, nil, fmt.Errorf("failed to create privateKey: %w", err)
	}

	txOpts, err := bind.NewKeyedTransactorWithChainID(privateKey, chainId)
	if err != nil {
		_ = RevertToAnvilSnapshot(client.Client(), snapshotID)
		return zero, nil, fmt.Errorf("failed to create TransactOpts: %w", err)
	}

	// build the self hosted deployment struct
	selfHostedApplicationFactoryAddress, err := config.GetContractsSelfHostedApplicationFactoryAddress()
	if err != nil {
		return zero, nil, fmt.Errorf("failed retrieve self hosted application factory address: %w", err)
	}

	selfHostedApplicationFactory, err := iselfhostedapplicationfactory.NewISelfHostedApplicationFactory(selfHostedApplicationFactoryAddress, client)
	if err != nil {
		return zero, nil, fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	applicationFactoryAddress, err := selfHostedApplicationFactory.GetApplicationFactory(nil)
	if err != nil {
		return zero, nil, err
	}

	authorityFactoryAddress, err := config.GetContractsAuthorityFactoryAddress()
	if err != nil {
		return zero, nil, err
	}

	ownerAddress := txOpts.From

	deployment := &SelfhostedDeployment{
		FactoryAddress:            selfHostedApplicationFactoryAddress,
		ApplicationFactoryAddress: applicationFactoryAddress,
		AuthorityFactoryAddress:   authorityFactoryAddress,
		OwnerAddress:              ownerAddress,
		TemplateHash:              templateHash,
		EpochLength:               10,
	}
	applicationAddress, _, err := deployment.Deploy(ctx, client, txOpts)

	// Define a cleanup function to revert to the snapshot
	cleanup := func() {
		err := RevertToAnvilSnapshot(client.Client(), snapshotID)
		if err != nil {
			log.Printf("failed to revert to snapshot: %v", err)
		}
	}

	return applicationAddress, cleanup, nil
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
