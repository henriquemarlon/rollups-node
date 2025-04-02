// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package execute

import (
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/ethutil"
)

var Cmd = &cobra.Command{
	Use:     "execute [app-name-or-address] [output-index]",
	Short:   "Executes a voucher",
	Example: examples,
	Args:    cobra.ExactArgs(2), // nolint: mnd
	Run:     run,
}

const examples = `# Executes voucher/output with index 5:
cartesi-rollups-cli execute echo-dapp 5

# Executes voucher/output with index 3 using application address:
cartesi-rollups-cli execute 0x1234567890123456789012345678901234567890 3

# Execute without confirmation prompt:
cartesi-rollups-cli execute echo-dapp 5 --yes`

var (
	blockchainHttpEndpoint string
	databaseConnection     string
	skipConfirmation       bool
)

func init() {
	Cmd.Flags().StringVar(&databaseConnection, "database-connection", "",
		"Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	cobra.CheckErr(viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.Flags().Lookup("database-connection")))

	Cmd.Flags().StringVar(&blockchainHttpEndpoint, "blockchain-http-endpoint", "", "Blockchain HTTP endpoint")
	cobra.CheckErr(viper.BindPFlag(config.BLOCKCHAIN_HTTP_ENDPOINT, Cmd.Flags().Lookup("blockchain-http-endpoint")))

	Cmd.Flags().BoolVarP(&skipConfirmation, "yes", "y", false, "Skip confirmation prompt")
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	nameOrAddress, err := config.ToApplicationNameOrAddressFromString(args[0])
	cobra.CheckErr(err)

	outputIndex, err := config.ToUint64FromDecimalOrHexString(args[1])
	cobra.CheckErr(err)

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	output, err := repo.GetOutput(ctx, nameOrAddress, outputIndex)
	cobra.CheckErr(err)

	if output == nil {
		fmt.Fprintf(os.Stderr, "The output with index %d was not found in the database\n", outputIndex)
		os.Exit(1)
	}

	app, err := repo.GetApplication(ctx, nameOrAddress)
	cobra.CheckErr(err)

	if len(output.OutputHashesSiblings) == 0 {
		fmt.Fprintf(os.Stderr, "The output with index %d has no associated proof yet\n", outputIndex)
		os.Exit(1)
	}

	client, err := ethclient.DialContext(ctx, ethEndpoint.String())
	cobra.CheckErr(err)

	chainId, err := client.ChainID(ctx)
	cobra.CheckErr(err)

	txOpts, err := auth.GetTransactOpts(chainId)
	cobra.CheckErr(err)

	if !skipConfirmation {
		fmt.Printf("Preparing to execute application %v (%v) output index %v with account %v\n",
			app.Name, app.IApplicationAddress, outputIndex, txOpts.From)

		fmt.Print("Do you want to continue? [y/N]: ")
		var response string
		_, err = fmt.Scanln(&response)
		if err != nil && err.Error() != "unexpected newline" {
			cobra.CheckErr(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Transaction cancelled")
			os.Exit(0)
		}
	}

	txHash, err := ethutil.ExecuteOutput(
		ctx,
		client,
		txOpts,
		app.IApplicationAddress,
		outputIndex,
		output.RawData,
		output.OutputHashesSiblings,
	)
	cobra.CheckErr(err)

	fmt.Printf("Voucher executed tx-hash: %v\n", txHash)
}
