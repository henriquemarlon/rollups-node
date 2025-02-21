// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package remove

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository/factory"
)

var Cmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove registered applications",
	Example: examples,
	Run:     run,
}

const examples = `# Get application status:
cartesi-rollups-cli app status -n echo-dapp`

var (
	name    string
	address string
)

func init() {
	Cmd.Flags().StringVarP(&name, "name", "n", "", "Application name")

	Cmd.Flags().StringVarP(&address, "address", "a", "", "Application contract address")

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

	if app.State == model.ApplicationState_Enabled {
		fmt.Fprintf(os.Stderr, "Error: Application %s is ENABLED. Must disable it first\n", app.Name)
		os.Exit(1)
	}

	err = repo.DeleteApplication(ctx, app.ID)
	cobra.CheckErr(err)

	fmt.Printf("Application %s successfully removed\n", app.Name)
}
