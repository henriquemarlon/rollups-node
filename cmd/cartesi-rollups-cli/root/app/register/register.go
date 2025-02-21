// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package register

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cartesi/rollups-node/internal/advancer/snapshot"
	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/ethutil"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:     "register",
	Short:   "Register an existing application on the node",
	Example: examples,
	Run:     run,
}

const examples = `# Adds an application to Rollups Node:
cartesi-rollups-cli app register -n echo-dapp -a 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF -t applications/echo-dapp`

var (
	name                   string
	applicationAddress     string
	consensusAddress       string
	templatePath           string
	templateHash           string
	epochLength            uint64
	inputBoxBlockNumber    uint64
	blockchainHttpEndpoint string
	enableMachineHashCheck bool
	disabled               bool
	printAsJSON            bool
)

func init() {
	Cmd.Flags().StringVarP(&name, "name", "n", "", "Application name")
	cobra.CheckErr(Cmd.MarkFlagRequired("name"))

	Cmd.Flags().StringVarP(&applicationAddress, "address", "a", "", "Application contract address")
	cobra.CheckErr(Cmd.MarkFlagRequired("address"))

	Cmd.Flags().StringVarP(&consensusAddress, "consensus", "c", "",
		"Application IConsensus Address. (DO NOT USE IN PRODUCTION)\nThis value is retrieved from the application contract",
	)

	Cmd.Flags().StringVarP(&templatePath, "template-path", "t", "", "Application template URI")
	cobra.CheckErr(Cmd.MarkFlagRequired("template-path"))

	Cmd.Flags().StringVarP(&templateHash, "template-hash", "H", "",
		"Application template hash. (DO NOT USE IN PRODUCTION)\nThis value is retrieved from the application contract",
	)

	Cmd.Flags().Uint64VarP(&inputBoxBlockNumber, "inputbox-block-number", "i", 0, "InputBox deployment block number")

	Cmd.Flags().Uint64VarP(&epochLength, "epoch-length", "e", 10,
		"Consensus Epoch length. (DO NOT USE IN PRODUCTION)\nThis value is retrieved from the consensus contract",
	)

	Cmd.Flags().BoolVarP(&disabled, "disabled", "d", false, "Sets the application state to disabled")

	Cmd.Flags().BoolVarP(&printAsJSON, "print-json", "j", false, "Prints the application data as JSON")

	Cmd.Flags().StringVar(&blockchainHttpEndpoint, "blockchain-http-endpoint", "", "Blockchain HTTP endpoint")
	viper.BindPFlag(config.BLOCKCHAIN_HTTP_ENDPOINT, Cmd.Flags().Lookup("blockchain-http-endpoint"))

	Cmd.Flags().BoolVar(&enableMachineHashCheck, "machine-hash-check", true, "Enable or disable machine hash check (DO NOT DISABLE IN PRODUCTION)")
	viper.BindPFlag(config.FEATURE_MACHINE_HASH_CHECK_ENABLED, Cmd.Flags().Lookup("machine-hash-check"))
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	applicationState := model.ApplicationState_Enabled
	if disabled {
		applicationState = model.ApplicationState_Disabled
	}

	address := common.HexToAddress(applicationAddress)

	var parsedTemplateHash common.Hash
	if cmd.Flags().Changed("template-hash") {
		parsedTemplateHash = common.HexToHash(templateHash)
	} else {
		contractTemplateHash, err := getTemplateHash(ctx, address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get template hash from application: %v\n", err)
			os.Exit(1)
		}
		parsedTemplateHash = *contractTemplateHash
		checkEnabled, err := config.GetFeatureMachineHashCheckEnabled()
		cobra.CheckErr(err)
		if checkEnabled {
			templateHash, err := snapshot.ReadHash(templatePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Read machine template hash failed: %v\n", err)
				os.Exit(1)
			}
			snapshotTemplateHash := common.HexToHash(templateHash)
			if parsedTemplateHash != snapshotTemplateHash {
				fmt.Fprintf(os.Stderr, "Template hash mismatch: contract has %s but machine has %s\n",
					contractTemplateHash.Hex(), parsedTemplateHash.Hex())
				os.Exit(1)
			}
		}
	}

	var consensus common.Address
	if cmd.Flags().Changed("consensus") {
		consensus = common.HexToAddress(consensusAddress)
	} else {
		consensus, err = getConsensus(ctx, address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get consensus address from application: %v\n", err)
			os.Exit(1)
		}
	}

	if !cmd.Flags().Changed("epochLength") {
		epochLength, err = getEpochLength(ctx, consensus)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get epoch length from consensus: %v\n", err)
			os.Exit(1)
		}
	}

	// TODO: inputBoxBlockNumber should come from config
	application := model.Application{
		Name:                 name,
		IApplicationAddress:  address,
		IConsensusAddress:    consensus,
		TemplateURI:          templatePath,
		TemplateHash:         parsedTemplateHash,
		EpochLength:          epochLength,
		State:                applicationState,
		LastProcessedBlock:   inputBoxBlockNumber,
		LastOutputCheckBlock: inputBoxBlockNumber,
		LastClaimCheckBlock:  inputBoxBlockNumber,
	}

	_, err = repo.CreateApplication(ctx, &application)
	cobra.CheckErr(err)

	if printAsJSON {
		jsonData, err := json.Marshal(application)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling application to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("Application %v successfully registered\n", application.IApplicationAddress)
	}
}

func getTemplateHash(
	ctx context.Context,
	appAddress common.Address,
) (*common.Hash, error) {
	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)
	client, err := ethclient.Dial(ethEndpoint.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the blockchain http endpoint: %s", ethEndpoint.Redacted())
	}
	return ethutil.GetTemplateHash(ctx, client, appAddress)
}

func getConsensus(
	ctx context.Context,
	appAddress common.Address,
) (common.Address, error) {
	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)
	client, err := ethclient.Dial(ethEndpoint.String())
	if err != nil {
		return common.Address{}, fmt.Errorf("Failed to connect to the blockchain http endpoint: %s", ethEndpoint.Redacted())
	}
	return ethutil.GetConsensus(ctx, client, appAddress)
}

func getEpochLength(
	ctx context.Context,
	consensusAddr common.Address,
) (uint64, error) {
	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)
	client, err := ethclient.Dial(ethEndpoint.String())
	if err != nil {
		return 0, fmt.Errorf("Failed to connect to the blockchain http endpoint: %s", ethEndpoint.Redacted())
	}
	return ethutil.GetEpochLength(ctx, client, consensusAddr)
}
