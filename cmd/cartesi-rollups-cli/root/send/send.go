// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package send

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/ethutil"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:     "send",
	Short:   "Send a rollups input to the Ethereum node",
	Example: examples,
	Run:     run,
}

const examples = `# Send the string "hi":
cartesi-rollups-cli send -n echo-dapp --payload "hi"

# Send the string "hi" encoded as hex:
cartesi-rollups-cli send -n echo-dapp --payload 0x6869 --hex

# Read from stdin:
echo "hi" | cartesi-rollups-cli send -n echo-dapp`

var (
	name                   string
	address                string
	blockchainHttpEndpoint string
	databaseConnection     string
	inputBoxAddress        string
	cmdPayload             string
	isHex                  bool
)

func init() {
	Cmd.Flags().StringVarP(&name, "name", "n", "", "Application name")

	Cmd.Flags().StringVarP(&address, "address", "a", "", "Application contract address")

	Cmd.Flags().StringVar(&cmdPayload, "payload", "", "input payload hex-encoded starting with 0x")

	Cmd.Flags().BoolVarP(&isHex, "hex", "x", false, "Force interpretation of --payload as hex.")

	Cmd.Flags().StringVar(&inputBoxAddress, "inputbox-address", "", "Input Box contract address")
	viper.BindPFlag(config.CONTRACTS_INPUT_BOX_ADDRESS, Cmd.Flags().Lookup("inputbox-address"))

	Cmd.Flags().StringVar(&databaseConnection, "database-connection", "", "Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.Flags().Lookup("database-connection"))

	Cmd.Flags().StringVar(&blockchainHttpEndpoint, "blockchain-http-endpoint", "", "Blockchain HTTP endpoint")
	viper.BindPFlag(config.BLOCKCHAIN_HTTP_ENDPOINT, Cmd.Flags().Lookup("blockchain-http-endpoint"))

	Cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if name == "" && address == "" {
			return fmt.Errorf("either 'name' or 'address' must be specified")
		}
		if name != "" && address != "" {
			return fmt.Errorf("only one of 'name' or 'address' can be specified")
		}
		// TODO check auth from env
		return nil
	}
}

func resolvePayload(cmd *cobra.Command) ([]byte, error) {
	if !cmd.Flags().Changed("payload") {
		stdinBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		return stdinBytes, nil
	}

	if isHex {
		return decodeHex(cmdPayload)
	}

	return []byte(cmdPayload), nil
}

func decodeHex(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		s = "0x" + s
	}

	b, err := hexutil.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex payload %q: %w", s, err)
	}
	return b, nil
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)

	iboxAddr, err := config.GetContractsInputBoxAddress()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	var nameOrAddress string
	if cmd.Flags().Changed("name") {
		nameOrAddress = name
	} else if cmd.Flags().Changed("address") {
		nameOrAddress = address
	}

	app, err := repo.GetApplication(ctx, nameOrAddress)
	cobra.CheckErr(err)
	if app == nil {
		fmt.Fprintf(os.Stderr, "application %q not found\n", nameOrAddress)
		os.Exit(1)
	}

	payload, err := resolvePayload(cmd)
	cobra.CheckErr(err)

	client, err := ethclient.DialContext(ctx, ethEndpoint.String())
	cobra.CheckErr(err)

	chainId, err := client.ChainID(ctx)
	cobra.CheckErr(err)

	txOpts, err := auth.GetTransactOpts(chainId)
	cobra.CheckErr(err)

	inputIndex, blockNumber, err := ethutil.AddInput(ctx, client, txOpts, iboxAddr, app.IApplicationAddress, payload)
	cobra.CheckErr(err)

	fmt.Printf("Input sent to app at %s. Index: %d BlockNumber: %d\n", app.IApplicationAddress, inputIndex, blockNumber)
}
