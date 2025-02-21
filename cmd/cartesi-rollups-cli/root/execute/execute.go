// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package execute

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/pkg/ethutil"
)

var Cmd = &cobra.Command{
	Use:     "execute",
	Short:   "Executes a voucher",
	Example: examples,
	Run:     run,
}

const examples = `# Executes voucher/output with index 5:
cartesi-rollups-cli execute -n echo-dapp --output-index 5`

var (
	name                   string
	address                string
	outputIndex            uint64
	blockchainHttpEndpoint string
	databaseConnection     string
)

func init() {
	Cmd.Flags().StringVarP(&name, "name", "n", "", "Application name")

	Cmd.Flags().StringVarP(&address, "address", "a", "", "Application contract address")

	Cmd.Flags().Uint64Var(&outputIndex, "output-index", 0, "Index of the output")
	cobra.CheckErr(Cmd.MarkFlagRequired("output-index"))

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
		return nil
	}
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	ethEndpoint, err := config.GetBlockchainHttpEndpoint()
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

	output, err := repo.GetOutput(ctx, nameOrAddress, outputIndex)
	cobra.CheckErr(err)

	if output == nil {
		fmt.Fprintf(os.Stderr, "The voucher/output with index %d was not found in the database\n", outputIndex)
		os.Exit(1)
	}

	app, err := repo.GetApplication(ctx, nameOrAddress)
	cobra.CheckErr(err)

	if len(output.OutputHashesSiblings) == 0 {
		fmt.Fprintf(os.Stderr, "The voucher/output with index %d has no associated proof yet\n", outputIndex)
		os.Exit(1)
	}

	client, err := ethclient.DialContext(ctx, ethEndpoint.String())
	cobra.CheckErr(err)

	chainId, err := client.ChainID(ctx)
	cobra.CheckErr(err)

	txOpts, err := auth.GetTransactOpts(chainId)
	cobra.CheckErr(err)

	fmt.Printf("Executing voucher app: %v (%v) output_index: %v with account: %v\n", app.Name, app.IApplicationAddress, outputIndex, txOpts.From)
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
