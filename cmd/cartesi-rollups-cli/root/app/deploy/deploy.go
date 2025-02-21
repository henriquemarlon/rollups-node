// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package deploy

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/cartesi/rollups-node/internal/advancer/snapshot"
	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/contracts/iapplicationfactory"
	"github.com/cartesi/rollups-node/pkg/contracts/iauthorityfactory"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:     "deploy",
	Short:   "Deploy an application and add it to the node",
	Example: examples,
	Run:     run,
}

const examples = `# Adds an application to Rollups Node:
cartesi-rollups-cli app deploy -n echo-dapp -a 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF -c 0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -t applications/echo-dapp` //nolint:lll

var (
	name                   string
	applicationOwner       string
	authorityOwner         string
	templatePath           string
	templateHash           string
	consensusAddr          string
	appFactoryAddr         string
	authorityFactoryAddr   string
	blockchainHttpEndpoint string
	salt                   string
	inputBoxBlockNumber    uint64
	epochLength            uint64
	disabled               bool
	printAsJSON            bool
	noRegister             bool
)

func init() {
	Cmd.Flags().StringVarP(&name, "name", "n", "", "Application name")
	cobra.CheckErr(Cmd.MarkFlagRequired("name"))

	Cmd.Flags().StringVarP(&applicationOwner, "app-owner", "o", "",
		"Application owner. If not defined, it will be derived from the auth method.",
	)

	Cmd.Flags().StringVarP(&authorityOwner, "authority-owner", "O", "",
		"Authority owner. If not defined, it will be derived from the auth method.",
	)

	Cmd.Flags().StringVarP(&templatePath, "template-path", "t", "", "Application template path")
	cobra.CheckErr(Cmd.MarkFlagRequired("template-path"))

	Cmd.Flags().StringVarP(&templateHash, "template-hash", "H", "",
		"Application template hash. If not provided, it will be read from the template path",
	)

	Cmd.Flags().StringVarP(&appFactoryAddr, "application-factory", "a", "", "Application Factory Address")
	viper.BindPFlag(config.CONTRACTS_APPLICATION_FACTORY_ADDRESS, Cmd.Flags().Lookup("application-factory"))

	Cmd.Flags().StringVarP(&consensusAddr, "consensus", "c", "", "Application IConsensus Address")

	Cmd.Flags().StringVarP(&authorityFactoryAddr, "authority-factory", "C", "",
		"Authority Factory Address. If defined, epoch-length value will be used to create a new consensus",
	)
	viper.BindPFlag(config.CONTRACTS_AUTHORITY_FACTORY_ADDRESS, Cmd.Flags().Lookup("authority-factory"))

	Cmd.Flags().Uint64VarP(&epochLength, "epoch-length", "e", 10,
		"Consensus Epoch length. If consensus address is provided, the value will be read from the contract",
	)

	Cmd.Flags().Uint64VarP(&inputBoxBlockNumber, "inputbox-block-number", "i", 0, "InputBox deployment block number")
	viper.BindPFlag(config.CONTRACTS_INPUT_BOX_DEPLOYMENT_BLOCK_NUMBER, Cmd.Flags().Lookup("inputbox-block-number"))

	Cmd.Flags().StringVar(&salt, "salt", "0000000000000000000000000000000000000000000000000000000000000000", "salt")

	Cmd.Flags().BoolVarP(&disabled, "disabled", "d", false, "Sets the application state to disabled")
	Cmd.Flags().BoolVar(&noRegister, "no-register", false, "Don't register the application on the node. Only deploy contracts")

	Cmd.Flags().BoolVarP(&printAsJSON, "print-json", "j", false, "Prints the application data as JSON")

	Cmd.Flags().StringVar(&blockchainHttpEndpoint, "blockchain-http-endpoint", "", "Blockchain HTTP endpoint")
	viper.BindPFlag(config.BLOCKCHAIN_HTTP_ENDPOINT, Cmd.Flags().Lookup("blockchain-http-endpoint"))
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	applicationState := model.ApplicationState_Enabled
	if disabled {
		applicationState = model.ApplicationState_Disabled
	}

	if templateHash == "" {
		var err error
		templateHash, err = snapshot.ReadHash(templatePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Read machine template hash failed: %v\n", err)
			os.Exit(1)
		}
	}

	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)

	client, err := ethclient.DialContext(ctx, ethEndpoint.String())
	cobra.CheckErr(err)

	chainId, err := client.ChainID(ctx)
	cobra.CheckErr(err)

	txOpts, err := auth.GetTransactOpts(chainId)
	cobra.CheckErr(err)

	var consensus common.Address
	if consensusAddr == "" {
		var owner common.Address
		authorityFactoryAddress, err := config.GetContractsAuthorityFactoryAddress()
		cobra.CheckErr(err)
		if cmd.Flags().Changed("authority-owner") {
			owner = common.HexToAddress(authorityOwner)
		} else {
			owner = txOpts.From
		}
		consensus, err = deployAuthority(ctx, client, txOpts, authorityFactoryAddress, owner, epochLength, salt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authoriy contract creation failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		consensus = common.HexToAddress(consensusAddr)
		epochLength, err = getEpochLength(consensus)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get epoch length from consensus: %v\n", err)
			os.Exit(1)
		}
	}

	var owner common.Address
	if cmd.Flags().Changed("application-owner") {
		owner = common.HexToAddress(applicationOwner)
	} else {
		owner = txOpts.From
	}
	applicationFactoryAddress, err := config.GetContractsApplicationFactoryAddress()
	cobra.CheckErr(err)
	appAddr, err := deployApplication(ctx, client, txOpts, applicationFactoryAddress, consensus, owner, templateHash, salt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Application contract creation failed: %v\n", err)
		os.Exit(1)
	}

	application := model.Application{
		Name:                 name,
		IApplicationAddress:  appAddr,
		IConsensusAddress:    consensus,
		TemplateURI:          templatePath,
		TemplateHash:         common.HexToHash(templateHash),
		EpochLength:          epochLength,
		State:                applicationState,
		LastProcessedBlock:   inputBoxBlockNumber,
		LastOutputCheckBlock: inputBoxBlockNumber,
		LastClaimCheckBlock:  inputBoxBlockNumber,
	}

	if !noRegister {
		dsn, err := config.GetDatabaseConnection()
		cobra.CheckErr(err)

		repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
		cobra.CheckErr(err)
		defer repo.Close()

		_, err = repo.CreateApplication(ctx, &application)
		cobra.CheckErr(err)
	}

	if printAsJSON {
		jsonData, err := json.Marshal(application)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling application to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("Application %v successfully deployed\n", application.IApplicationAddress)
	}
}

// FIXME remove this
func deployApplication(
	ctx context.Context,
	client *ethclient.Client,
	txOpts *bind.TransactOpts,
	applicationFactoryAddr common.Address,
	authorityAddr common.Address,
	owner common.Address,
	templateHash string,
	salt string,
) (common.Address, error) {

	templateHashBytes, err := hex.DecodeString(templateHash)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to decode template hash: %v", err)
	}
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to decode salt: %v", err)
	}

	factory, err := iapplicationfactory.NewIApplicationFactory(applicationFactoryAddr, client)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	tx, err := factory.NewApplication(txOpts, authorityAddr, owner, toBytes32(templateHashBytes), toBytes32(saltBytes))
	if err != nil {
		return common.Address{}, fmt.Errorf("Transaction failed: %v", err)
	}

	if !printAsJSON {
		fmt.Printf("Transaction submitted: %s\n", tx.Hash().Hex())
	}

	// Wait for the transaction to be mined
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to wait for transaction mining: %v", err)
	}

	if receipt.Status == 1 {
		if !printAsJSON {
			fmt.Println("Transaction successful!")
		}
	} else {
		return common.Address{}, fmt.Errorf("Transaction failed!")
	}

	// Parse logs to get the address of the new application contract
	contractABI, err := abi.JSON(strings.NewReader(iapplicationfactory.IApplicationFactoryABI))
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to parse ABI: %v", err)
	}

	// Look for the specific event in the receipt logs
	for _, vLog := range receipt.Logs {
		event := struct {
			Consensus    common.Address
			AppOwner     common.Address
			TemplateHash [32]byte
			AppContract  common.Address
		}{}

		// Parse log for ApplicationCreated event
		err := contractABI.UnpackIntoInterface(&event, "ApplicationCreated", vLog.Data)
		if err != nil {
			continue // Skip logs that don't match
		}

		if !printAsJSON {
			fmt.Printf("New Application contract deployed at address: %s\n", event.AppContract.Hex())
		}
		return event.AppContract, nil
	}

	return common.Address{}, fmt.Errorf("failed to find ApplicationCreated event in receipt logs")
}

// FIXME remove this
func deployAuthority(
	ctx context.Context,
	client *ethclient.Client,
	txOpts *bind.TransactOpts,
	authorityFactoryAddr common.Address,
	owner common.Address,
	epochLength uint64,
	salt string,
) (common.Address, error) {
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to decode salt: %v", err)
	}

	contract, err := iauthorityfactory.NewIAuthorityFactory(authorityFactoryAddr, client)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	tx, err := contract.NewAuthority0(txOpts, owner, big.NewInt(int64(epochLength)), toBytes32(saltBytes))
	if err != nil {
		return common.Address{}, fmt.Errorf("Transaction failed: %v", err)
	}

	if !printAsJSON {
		fmt.Printf("Transaction submitted: %s\n", tx.Hash().Hex())
	}

	// Wait for the transaction to be mined
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to wait for transaction mining: %v", err)
	}

	if receipt.Status == 1 {
		if !printAsJSON {
			fmt.Println("Transaction successful!")
		}
	} else {
		return common.Address{}, fmt.Errorf("Transaction failed!")
	}

	// Parse logs to get the address of the new application contract
	contractABI, err := abi.JSON(strings.NewReader(iauthorityfactory.IAuthorityFactoryABI))
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to parse ABI: %v", err)
	}

	// Look for the specific event in the receipt logs
	for _, vLog := range receipt.Logs {
		event := struct {
			Authority common.Address
		}{}

		// Parse log for ApplicationCreated event
		err := contractABI.UnpackIntoInterface(&event, "AuthorityCreated", vLog.Data)
		if err != nil {
			continue // Skip logs that don't match
		}

		if !printAsJSON {
			fmt.Printf("New Authority contract deployed at address: %s\n", event.Authority.Hex())
		}
		return event.Authority, nil
	}

	return common.Address{}, fmt.Errorf("failed to find AuthorityCreated event in receipt logs")
}

func getEpochLength(
	consensusAddr common.Address,
) (uint64, error) {
	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)

	client, err := ethclient.Dial(ethEndpoint.String())
	if err != nil {
		return 0, fmt.Errorf("Failed to connect to the blockchain http endpoint: %s", ethEndpoint.Redacted())
	}

	consensus, err := iconsensus.NewIConsensus(consensusAddr, client)
	if err != nil {
		return 0, fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	epochLengthRaw, err := consensus.GetEpochLength(nil)
	if err != nil {
		return 0, fmt.Errorf("error retrieving application epoch length: %v", err)
	}
	return epochLengthRaw.Uint64(), nil
}

func toBytes32(data []byte) [32]byte {
	var arr [32]byte
	if len(data) != 32 {
		fmt.Fprintf(os.Stderr, "Invalid length: expected 32 bytes, got %d bytes", len(data))
		os.Exit(1)
	}
	copy(arr[:], data)
	return arr
}
