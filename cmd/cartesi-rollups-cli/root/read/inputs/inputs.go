// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package inputs

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/contracts/inputs"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "inputs [application-name-or-address] [input-index]",
	Short:   "Reads inputs ordered by index",
	Example: examples,
	Args:    cobra.RangeArgs(1, 2), // nolint: mnd
	Run:     run,
	Long: `
Supported Environment Variables:
  CARTESI_DATABASE_CONNECTION                    Database connection string`,
}

const examples = `# Read all inputs:
cartesi-rollups-cli read inputs echo-dapp

# Read specific input by index:
cartesi-rollups-cli read inputs echo-dapp 42

# Read with decoded input data:
cartesi-rollups-cli read inputs echo-dapp 42 --decode`

var (
	decodeInput bool
)

func init() {
	Cmd.Flags().BoolVarP(&decodeInput, "decode", "d", false,
		"Prints the decoded input RawData")

	origHelpFunc := Cmd.HelpFunc()
	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().Lookup("verbose").Hidden = false
		command.Flags().Lookup("database-connection").Hidden = false
		origHelpFunc(command, strings)
	})
}

type EvmAdvance struct {
	ChainId        string `json:"chainId"`
	AppContract    string `json:"appContract"`
	MsgSender      string `json:"msgSender"`
	BlockNumber    string `json:"blockNumber"`
	BlockTimestamp string `json:"blockTimestamp"`
	PrevRandao     string `json:"prevRandao"`
	Index          string `json:"index"`
	Payload        string `json:"payload"`
}

func decodeInputData(input *model.Input, parsedAbi *abi.ABI) (EvmAdvance, error) {
	decoded := make(map[string]any)
	err := parsedAbi.Methods["EvmAdvance"].Inputs.UnpackIntoMap(decoded, input.RawData[4:])
	if err != nil {
		return EvmAdvance{}, err
	}

	params := EvmAdvance{
		ChainId:        decoded["chainId"].(*big.Int).String(),
		AppContract:    decoded["appContract"].(common.Address).Hex(),
		MsgSender:      decoded["msgSender"].(common.Address).Hex(),
		BlockNumber:    decoded["blockNumber"].(*big.Int).String(),
		BlockTimestamp: decoded["blockTimestamp"].(*big.Int).String(),
		PrevRandao:     decoded["prevRandao"].(*big.Int).String(),
		Index:          decoded["index"].(*big.Int).String(),
		Payload:        "0x" + hex.EncodeToString(decoded["payload"].([]byte)),
	}

	return params, nil
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	nameOrAddress, err := config.ToApplicationNameOrAddressFromString(args[0])
	cobra.CheckErr(err)

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	var result []byte
	if len(args) == 1 {
		// List all inputs
		inputList, _, err := repo.ListInputs(ctx, nameOrAddress, repository.InputFilter{}, repository.Pagination{})
		cobra.CheckErr(err)

		if decodeInput {
			parsedAbi, err := inputs.InputsMetaData.GetAbi()
			cobra.CheckErr(err)

			var decodedInputs []EvmAdvance
			for _, input := range inputList {
				params, err := decodeInputData(input, parsedAbi)
				cobra.CheckErr(err)
				decodedInputs = append(decodedInputs, params)
			}

			result, err = json.MarshalIndent(decodedInputs, "", "  ")
			cobra.CheckErr(err)
		} else {
			result, err = json.MarshalIndent(inputList, "", "    ")
			cobra.CheckErr(err)
		}
	} else {
		// Get specific input by index
		inputIndex, err := config.ToUint64FromDecimalOrHexString(args[1])
		if err != nil {
			cobra.CheckErr(fmt.Errorf("invalid value for input-index: %w", err))
		}

		input, err := repo.GetInput(ctx, nameOrAddress, inputIndex)
		cobra.CheckErr(err)

		if decodeInput {
			parsedAbi, err := inputs.InputsMetaData.GetAbi()
			cobra.CheckErr(err)

			params, err := decodeInputData(input, parsedAbi)
			cobra.CheckErr(err)

			result, err = json.MarshalIndent(params, "", "  ")
			cobra.CheckErr(err)
		} else {
			result, err = json.MarshalIndent(input, "", "    ")
			cobra.CheckErr(err)
		}
	}

	fmt.Println(string(result))
}
