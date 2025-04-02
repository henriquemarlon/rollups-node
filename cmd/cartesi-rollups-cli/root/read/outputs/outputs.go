// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package outputs

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/contracts/outputs"
)

var Cmd = &cobra.Command{
	Use:     "outputs [application-name-or-address] [output-index]",
	Short:   "Reads outputs",
	Example: examples,
	Args:    cobra.RangeArgs(1, 2), // nolint: mnd
	Run:     run,
}

const examples = `# Read all outputs:
cartesi-rollups-cli read outputs echo-dapp

# Read specific output by index:
cartesi-rollups-cli read outputs echo-dapp 42

# Read all outputs filtering by input index:
cartesi-rollups-cli read outputs echo-dapp --input-index=23

# Read outputs with decoded data:
cartesi-rollups-cli read outputs echo-dapp --decode`

var (
	inputIndex   uint64
	decodeOutput bool
)

func init() {
	Cmd.Flags().Uint64Var(&inputIndex, "input-index", 0,
		"filter outputs by input index")

	Cmd.Flags().BoolVarP(&decodeOutput, "decode", "d", false,
		"prints the decoded Output RawData")
}

type Notice struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type Voucher struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Value       string `json:"value"`
	Payload     string `json:"payload"`
}

type DelegateCallVoucher struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Payload     string `json:"payload"`
}

func decodeOutputData(output *model.Output, parsedAbi *abi.ABI) (any, error) {
	if len(output.RawData) < 4 { // nolint: mnd
		return nil, fmt.Errorf("raw data too short")
	}

	method, err := parsedAbi.MethodById(output.RawData[:4])
	if err != nil {
		return nil, err
	}

	decoded := make(map[string]any)
	if err := method.Inputs.UnpackIntoMap(decoded, output.RawData[4:]); err != nil {
		return nil, fmt.Errorf("failed to unpack %s: %w", method.Name, err)
	}

	switch method.Name {
	case "Notice":
		payload, ok := decoded["payload"].([]byte)
		if !ok {
			return nil, fmt.Errorf("unable to decode Notice payload")
		}
		return Notice{
			Type:    "Notice",
			Payload: "0x" + hex.EncodeToString(payload),
		}, nil

	case "Voucher":
		dest, ok1 := decoded["destination"].(common.Address)
		value, ok2 := decoded["value"].(*big.Int)
		payload, ok3 := decoded["payload"].([]byte)
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("unable to decode Voucher parameters")
		}
		return Voucher{
			Type:        "Voucher",
			Destination: dest.Hex(),
			Value:       value.String(),
			Payload:     "0x" + hex.EncodeToString(payload),
		}, nil

	case "DelegateCallVoucher":
		dest, ok1 := decoded["destination"].(common.Address)
		payload, ok2 := decoded["payload"].([]byte)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("unable to decode DelegateCallVoucher parameters")
		}
		return DelegateCallVoucher{
			Type:        "DelegateCallVoucher",
			Destination: dest.Hex(),
			Payload:     "0x" + hex.EncodeToString(payload),
		}, nil

	default:
		return map[string]string{
			"type":    method.Name,
			"rawData": "0x" + hex.EncodeToString(output.RawData),
		}, nil
	}
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
	if len(args) == 2 { // nolint: mnd
		// Get a specific output by index
		outputIndex, err := config.ToUint64FromDecimalOrHexString(args[1])
		if err != nil {
			cobra.CheckErr(fmt.Errorf("invalid output index value: %w", err))
		}

		output, err := repo.GetOutput(ctx, nameOrAddress, outputIndex)
		cobra.CheckErr(err)

		if decodeOutput {
			parsedAbi, err := outputs.OutputsMetaData.GetAbi()
			cobra.CheckErr(err)

			decoded, err := decodeOutputData(output, parsedAbi)
			cobra.CheckErr(err)

			result, err = json.MarshalIndent(decoded, "", "    ")
			cobra.CheckErr(err)
		} else {
			result, err = json.MarshalIndent(output, "", "    ")
			cobra.CheckErr(err)
		}
	} else {
		// List outputs with optional input index filter
		f := repository.OutputFilter{}
		if cmd.Flags().Changed("input-index") {
			inputIndexPtr := &inputIndex
			f.InputIndex = inputIndexPtr
		}

		p := repository.Pagination{}
		outputList, _, err := repo.ListOutputs(ctx, nameOrAddress, f, p)
		cobra.CheckErr(err)

		if decodeOutput {
			parsedAbi, err := outputs.OutputsMetaData.GetAbi()
			cobra.CheckErr(err)

			var decodedOutputs []any
			for _, output := range outputList {
				decoded, err := decodeOutputData(output, parsedAbi)
				if err != nil {
					// If decoding fails, include the error with the raw data.
					decoded = map[string]string{
						"type":    "unknown",
						"rawData": "0x" + hex.EncodeToString(output.RawData),
						"error":   err.Error(),
					}
				}
				decodedOutputs = append(decodedOutputs, decoded)
			}

			// Marshal the decoded outputs as indented JSON.
			result, err = json.MarshalIndent(decodedOutputs, "", "  ")
			cobra.CheckErr(err)
		} else {
			result, err = json.MarshalIndent(outputList, "", "    ")
			cobra.CheckErr(err)
		}
	}

	fmt.Println(string(result))
}
