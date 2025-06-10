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
	"path"
	"strings"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/contracts/dataavailability"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"
	"github.com/cartesi/rollups-node/pkg/contracts/iinputbox"
	"github.com/cartesi/rollups-node/pkg/ethutil"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

var (
	applicationConsensusAddressParam         string
	applicationDataAvailabilityParam         string
	applicationFactoryAddressParam           string
	applicationOwnerAddressParam             string
	applicationRegisterParam                 bool
	applicationSelfHostedParam               bool
	applicationTemplateHashParam             string
	applicationEnableParam                   bool
	selfHostedApplicationFactoryAddressParam string
)

var applicationCmd = &cobra.Command{
	Use:   "application <application-name> [template-path]",
	Short: "Deploy a new application and register it into the database",
	Args: func(cmd *cobra.Command, args []string) error {
		if !(1 <= len(args) && len(args) <= 2) {
			return fmt.Errorf("Expected 1 or 2 args")
		}
		return cobra.OnlyValidArgs(cmd, args)
	},
	Example: applicationExamples,
	Run:     runDeployApplication,
	Long: `
Supported Environment Variables:
  CARTESI_DATABASE_CONNECTION                                Database connection string
  CARTESI_BLOCKCHAIN_HTTP_ENDPOINT                           Blockchain HTTP endpoint
  CARTESI_CONTRACTS_INPUT_BOX_ADDRESS                        Input Box contract address
  CARTESI_CONTRACTS_APPLICATION_FACTORY_ADDRESS              Application Factory address
  CARTESI_CONTRACTS_AUTHORITY_FACTORY_ADDRESS                Authority Factory address
  CARTESI_CONTRACTS_SELF_HOSTED_APPLICATION_FACTORY_ADDRESS  Self Hosted Application Factory address`,
}

const applicationExamples = `
# deploy both application and authority contracts (separately), then register the application
 - cli deploy application echo-dapp applications/echo-dapp/

# deploy an application contract using an existing consensus, then register the application
 - cli deploy application echo-dapp applications/echo-dapp/ --consensus=0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

# deploy both application and authority contracts together via self hosted contract, then register the application
 - cli deploy application echo-dapp applications/echo-dapp/ --self-hosted

# deploy but don't register into the database
 - cli deploy application echo-dapp applications/echo-dapp/ --register=false

# deploy and register into the database, but disabled
 - cli deploy application echo-dapp applications/echo-dapp/ --enable=false

# deploy an application without a template
 - cli deploy application echo-dapp --template-hash=0x0000000000000000000000000000000000000000000000000000000000000000 --register=false`

func init() {
	applicationCmd.Flags().StringVarP(&applicationConsensusAddressParam, "consensus", "c", "",
		"Consensus address. A new authority consensus will be created if this field is left empty.")
	applicationCmd.Flags().StringVarP(&applicationFactoryAddressParam, "factory", "f", "",
		"Application factory address. Default value is retrieved from configuration.")
	applicationCmd.Flags().StringVarP(&applicationOwnerAddressParam, "owner", "o", "",
		"Application owner address. If not defined, it will be derived from the auth method.")
	applicationCmd.Flags().StringVarP(&applicationDataAvailabilityParam, "data-availability", "d", "",
		"Data availability string. Default is input box.")
	applicationCmd.Flags().BoolVarP(&applicationSelfHostedParam, "self-hosted", "s", false,
		"Self Hosted Application. Request the application to be deployed as self hosted.")
	applicationCmd.Flags().StringVarP(&applicationTemplateHashParam, "template-hash", "H", "",
		"Template hash. If not provided, it will be read from the template path")
	applicationCmd.Flags().BoolVarP(&applicationRegisterParam, "register", "r", true,
		"Register the application into the database")
	applicationCmd.Flags().BoolVarP(&applicationEnableParam, "enable", "e", true,
		"Start processing the application, requires 'register=true'.")

	// in case the user also requests an authority deployment
	applicationCmd.Flags().StringVarP(&authorityFactoryAddressParam, "authority-factory", "F", "",
		"Authority Factory Address. If defined, epoch-length value will be used to create a new consensus")
	applicationCmd.Flags().StringVarP(&authorityOwnerAddressParam, "authority-owner", "O", "",
		"Authority Owner. If not defined, it will be derived from the auth method.")

	origHelpFunc := applicationCmd.HelpFunc()
	applicationCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().Lookup("epoch-length").Hidden = false
		command.Flags().Lookup("salt").Hidden = false
		command.Flags().Lookup("json").Hidden = false
		command.Flags().Lookup("verbose").Hidden = false
		origHelpFunc(command, strings)
	})
}

func runDeployApplication(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)

	client, err := ethclient.DialContext(ctx, ethEndpoint.String())
	cobra.CheckErr(err)

	chainId, err := client.ChainID(ctx)
	cobra.CheckErr(err)

	txOpts, err := auth.GetTransactOpts(chainId)
	cobra.CheckErr(err)

	deployment, err := buildApplicationDeployment(cmd, args, ctx, client, txOpts)
	cobra.CheckErr(err)

	if !asJson {
		fmt.Printf("\nDeploying application: %v...", deployment.Application.Name)
	}
	deployment.IApplicationAddress, err = deployment.Deploy(ctx, client, txOpts)
	if err != nil {
		if asJson {
			result := struct {
				Code    int
				Message string
			}{
				Code:    1,
				Message: err.Error(),
			}
			report, err := json.MarshalIndent(&result, "", "  ")
			cobra.CheckErr(err)
			fmt.Println(string(report))
		} else {
			fmt.Printf("failure\n\n")
			fmt.Fprintf(os.Stderr, "%v.\n\n", err)
		}
		os.Exit(1)
	}
	if !asJson {
		fmt.Printf("success\n\n")
		fmt.Println("application address:", deployment.IApplicationAddress)
		if deployment.WithSelfHosted != nil {
			fmt.Println("consensus address:", deployment.IConsensusAddress)
		} else if deployment.WithAuthority != nil {
			fmt.Println("consensus address:", deployment.IConsensusAddress)
		}
		if verbose {
			if deployment.WithSelfHosted != nil {
				fmt.Println("selfhosted factory:", deployment.WithSelfHosted.FactoryAddress)
				fmt.Println("application factory:", deployment.WithSelfHosted.ApplicationFactoryAddress)
				fmt.Println("authority factory:", deployment.WithSelfHosted.AuthorityFactoryAddress)
			} else if deployment.WithAuthority != nil {
				fmt.Println("application factory:", deployment.FactoryAddress)
				fmt.Println("authority factory:", deployment.WithAuthority.FactoryAddress)
			}
		}
	}

	// register
	registered := false
	if applicationRegisterParam {
		if !asJson {
			fmt.Printf("\nRegistering application: %v...", deployment.Application.Name)
		}

		if deployment.Application.TemplateURI == "" {
			if !asJson {
				fmt.Printf("failure. template-path is empty\n\n")
			}
		} else {
			dsn, err := config.GetDatabaseConnection()
			cobra.CheckErr(err)

			repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
			cobra.CheckErr(err)
			defer repo.Close()

			_, err = repo.CreateApplication(ctx, &deployment.Application)
			cobra.CheckErr(err)

			// retrieve fields initialized by the database
			app, err := repo.GetApplication(ctx, deployment.Application.Name)
			cobra.CheckErr(err)

			deployment.Application = *app
			if !asJson {
				fmt.Printf("success\n\n")
			}
			registered = true
		}
	}

	if asJson {
		if !verbose {
			report, err := json.MarshalIndent(&deployment.Application, "", "  ")
			cobra.CheckErr(err)
			fmt.Println(string(report))
		} else {
			if deployment.WithSelfHosted != nil {
				// application + selfhosted details
				deploymentReport := struct {
					Application        model.Application `json:"application"`
					Registered         bool              `json:"registered"`
					Owner              common.Address    `json:"owner"`
					SelfhostedFactory  common.Address    `json:"selfhosted_factory"`
					ApplicationFactory common.Address    `json:"application_factory"`
					AuthorityFactory   common.Address    `json:"authority_factory"`
				}{
					Application:        deployment.Application,
					Registered:         registered,
					Owner:              deployment.OwnerAddress,
					SelfhostedFactory:  deployment.FactoryAddress,
					ApplicationFactory: deployment.WithSelfHosted.ApplicationFactoryAddress,
					AuthorityFactory:   deployment.WithSelfHosted.AuthorityFactoryAddress,
				}
				report, err := json.MarshalIndent(&deploymentReport, "", "  ")
				cobra.CheckErr(err)
				fmt.Println(string(report))

			} else if deployment.WithAuthority != nil {
				// application + authority details
				deploymentReport := struct {
					Application        model.Application `json:"application"`
					Registered         bool              `json:"registered"`
					Owner              common.Address    `json:"owner"`
					ApplicationFactory common.Address    `json:"application_factory"`
					AuthorityFactory   common.Address    `json:"authority_factory"`
				}{
					Application:        deployment.Application,
					Registered:         registered,
					Owner:              deployment.OwnerAddress,
					ApplicationFactory: deployment.FactoryAddress,
					AuthorityFactory:   deployment.WithAuthority.FactoryAddress,
				}
				report, err := json.MarshalIndent(&deploymentReport, "", "  ")
				cobra.CheckErr(err)
				fmt.Println(string(report))

			} else {
				// application only
				deploymentReport := struct {
					Application        model.Application `json:"application"`
					Registered         bool              `json:"registered"`
					Owner              common.Address    `json:"owner"`
					ApplicationFactory common.Address    `json:"application_factory"`
				}{
					Application:        deployment.Application,
					Registered:         registered,
					Owner:              deployment.OwnerAddress,
					ApplicationFactory: deployment.FactoryAddress,
				}
				report, err := json.MarshalIndent(&deploymentReport, "", "  ")
				cobra.CheckErr(err)
				fmt.Println(string(report))
			}
		}
	}
}

// parse cmd+args, then build the deployment structure
func buildApplicationDeployment(
	cmd *cobra.Command,
	args []string,
	ctx context.Context,
	client *ethclient.Client,
	txOpts *bind.TransactOpts,
) (
	*ethutil.ApplicationDeployment,
	error,
) {
	var applicationFactoryAddress common.Address
	var applicationOwnerAddress common.Address
	var inputBoxAddress common.Address
	var inputBoxBlock *big.Int
	var encodedDA []byte
	var templateHash common.Hash
	var templatePath string

	name, err := config.ToApplicationNameFromString(args[0])
	if err != nil {
		return nil, err
	}

	if len(args) > 1 {
		templatePath = args[1]
	}

	if !cmd.Flags().Changed("template-hash") {
		if len(args) < 2 {
			return nil, fmt.Errorf("template-hash auto detection requires a value template-path")
		}
		templateHash, err = readHash(templatePath)
	} else {
		templateHash, err = parseHexHash(applicationTemplateHashParam)
	}
	if err != nil {
		return nil, err
	}

	if !cmd.Flags().Changed("application-factory") {
		applicationFactoryAddress, err = config.GetContractsApplicationFactoryAddress()
	} else {
		applicationFactoryAddress, err = parseHexAddress(applicationFactoryAddressParam)
	}
	if err != nil {
		return nil, err
	}

	if !cmd.Flags().Changed("application-owner") {
		applicationOwnerAddress = txOpts.From
	} else {
		applicationOwnerAddress, err = parseHexAddress(applicationOwnerAddressParam)
		if err != nil {
			return nil, err
		}
	}

	if !cmd.Flags().Changed("data-availability") {
		inputBoxAddress, inputBoxBlock, encodedDA, err = defaultDA(client)
	} else {
		inputBoxAddress, inputBoxBlock, encodedDA, err = customDA(client, applicationDataAvailabilityParam)
	}
	if err != nil {
		return nil, err
	}

	salt, err := ethutil.ParseSalt(saltParam)
	if err != nil {
		return nil, err
	}

	// partial construction of deployment. consensus will be updated after contracts are deployed
	deployment := ethutil.ApplicationDeployment{
		Application: model.Application{
			Name:             name,
			TemplateURI:      templatePath,
			TemplateHash:     templateHash,
			IInputBoxAddress: inputBoxAddress,
			IInputBoxBlock:   inputBoxBlock.Uint64(),
			DataAvailability: encodedDA,
			EpochLength:      epochLengthParam,
			State:            model.ApplicationState_Disabled,
		},
		FactoryAddress: applicationFactoryAddress,
		OwnerAddress:   applicationOwnerAddress,
		Salt:           salt,
	}

	if applicationEnableParam {
		deployment.State = model.ApplicationState_Enabled
	}

	if !cmd.Flags().Changed("consensus") {
		if applicationSelfHostedParam {
			deployment.WithSelfHosted, err = buildSelfhostedDeployment(cmd, args, &deployment)
		} else {
			deployment.WithAuthority, err = buildAuthorityDeployment(cmd, txOpts)
		}
	} else {
		deployment.IConsensusAddress, deployment.EpochLength, err = customConsensus(client, applicationConsensusAddressParam)
	}
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

// read the hash value from the cartesi machine hash file
func readHash(machineDir string) (common.Hash, error) {
	zero := common.Hash{}
	path := path.Join(machineDir, "hash")
	hash, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("read hash: %w", err)
	} else if len(hash) != common.HashLength {
		return zero, fmt.Errorf(
			"read hash: wrong size; expected %v bytes but read %v",
			common.HashLength,
			len(hash),
		)
	}
	return common.BytesToHash(hash), nil
}

func parseHexHash(hash string) (common.Hash, error) {
	out := common.Hash{}
	return out, out.UnmarshalText([]byte(hash))
}

// default DA is InputBox
func defaultDA(client *ethclient.Client) (common.Address, *big.Int, []byte, error) {
	parsedABI, err := dataavailability.DataAvailabilityMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to get data availability ABI: %w", err)
	}

	inputBoxAddress, err := config.GetContractsInputBoxAddress()
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to get contract input box address: %w", err)
	}

	encodedDA, err := parsedABI.Pack("InputBox", inputBoxAddress)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed pack input box data availability string with: %w", err)
	}

	inputBox, err := iinputbox.NewIInputBox(inputBoxAddress, client)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to create input box instance: %w", err)
	}

	inputBoxBlock, err := inputBox.GetDeploymentBlockNumber(nil)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to get deployment block number: %w", err)
	}

	return inputBoxAddress, inputBoxBlock, encodedDA, nil
}

func customDA(client *ethclient.Client, dataAvailability string) (common.Address, *big.Int, []byte, error) {
	parsedAbi, err := dataavailability.DataAvailabilityMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to get ABI: %w", err)
	}

	if len(dataAvailability) < 3 || (!strings.HasPrefix(dataAvailability, "0x") && !strings.HasPrefix(dataAvailability, "0X")) {
		return common.Address{}, nil, nil, fmt.Errorf("data Availability should be an ABI encoded value")
	}

	s := dataAvailability[2:]
	encodedDA, err := hex.DecodeString(s)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("error parsing Data Availability value: %w", err)
	}

	if len(encodedDA) < model.DATA_AVAILABILITY_SELECTOR_SIZE {
		return common.Address{}, nil, nil, fmt.Errorf("invalid Data Availability")
	}

	method, err := parsedAbi.MethodById(encodedDA[:model.DATA_AVAILABILITY_SELECTOR_SIZE])
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to get method by ID: %w", err)
	}

	args, err := method.Inputs.Unpack(encodedDA[model.DATA_AVAILABILITY_SELECTOR_SIZE:])
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to unpack inputs: %w", err)
	}

	if len(args) == 0 {
		return common.Address{}, nil, nil, fmt.Errorf("invalid Data Availability. Should at least contain InputBox Address")
	}

	var inputBoxAddress common.Address
	switch addr := args[0].(type) {
	case common.Address:
		inputBoxAddress = addr
	default:
		return common.Address{}, nil, nil, fmt.Errorf("first argument in Data Availability is not an address (got %T)", args[0])
	}

	inputbox, err := iinputbox.NewIInputBox(inputBoxAddress, client)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to create input box instance: %w", err)
	}

	inputBoxBlock, err := inputbox.GetDeploymentBlockNumber(nil)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to get deployment block number: %w", err)
	}

	return inputBoxAddress, inputBoxBlock, encodedDA, nil
}

func customConsensus(client *ethclient.Client, consensusString string) (common.Address, uint64, error) {
	consensusAddress, err := parseHexAddress(consensusString)
	if err != nil {
		return common.Address{}, 0, err
	}

	consensus, err := iconsensus.NewIConsensus(consensusAddress, client)
	if err != nil {
		return common.Address{}, 0, err
	}

	epochLengthBig, err := consensus.GetEpochLength(nil)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to retrieve consensus epoch length: %v", err)
	}

	return consensusAddress, epochLengthBig.Uint64(), nil
}
