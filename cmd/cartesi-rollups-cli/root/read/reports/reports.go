// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package reports

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/repository/factory"
)

var Cmd = &cobra.Command{
	Use:     "reports [application-name-or-address] [report-index]",
	Short:   "Reads reports",
	Example: examples,
	Args:    cobra.RangeArgs(1, 2), // nolint: mnd
	Run:     run,
	Long: `
Supported Environment Variables:
  CARTESI_DATABASE_CONNECTION                    Database connection string`,
}

const examples = `# Read all reports:
cartesi-rollups-cli read reports echo-dapp

# Read specific report by index:
cartesi-rollups-cli read reports echo-dapp 42

# Read all reports filtering by input index:
cartesi-rollups-cli read reports echo-dapp --input-index=23`

var (
	inputIndex uint64
)

func init() {
	Cmd.Flags().Uint64Var(&inputIndex, "input-index", 0,
		"filter reports by input index")

	origHelpFunc := Cmd.HelpFunc()
	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().Lookup("verbose").Hidden = false
		command.Flags().Lookup("database-connection").Hidden = false
		origHelpFunc(command, strings)
	})
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
		// Get a specific report by index
		reportIndex, err := config.ToUint64FromDecimalOrHexString(args[1])
		if err != nil {
			cobra.CheckErr(fmt.Errorf("invalid report index value: %w", err))
		}

		report, err := repo.GetReport(ctx, nameOrAddress, reportIndex)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(report, "", "    ")
		cobra.CheckErr(err)
	} else {
		// List reports with optional input index filter
		f := repository.ReportFilter{}
		if cmd.Flags().Changed("input-index") {
			inputIndexPtr := &inputIndex
			f.InputIndex = inputIndexPtr
		}

		p := repository.Pagination{}
		reports, _, err := repo.ListReports(ctx, nameOrAddress, f, p)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(reports, "", "    ")
		cobra.CheckErr(err)
	}

	fmt.Println(string(result))
}
