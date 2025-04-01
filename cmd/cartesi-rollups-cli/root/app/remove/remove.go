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
	Use:     "remove [app-name-or-address]",
	Aliases: []string{"rm"},
	Short:   "Remove registered applications",
	Example: examples,
	Args:    cobra.ExactArgs(1),
	Run:     run,
}

const examples = `# Remove application:
cartesi-rollups-cli app remove echo-dapp

# Remove application without confirmation:
cartesi-rollups-cli app remove echo-dapp --force`

var force bool

func init() {
	Cmd.Flags().BoolVarP(&force, "force", "f", false, "Force removal without confirmation")
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

	if !force {
		fmt.Printf("Are you sure you want to remove application %s (%s)? [y/N] ",
			app.Name, app.IApplicationAddress.String())

		var response string
		_, err = fmt.Scanln(&response)
		if err != nil || (response != "y" && response != "Y") {
			fmt.Println("Operation cancelled")
			return
		}
	}

	err = repo.DeleteApplication(ctx, app.ID)
	cobra.CheckErr(err)

	fmt.Printf("Application %s (%s) successfully removed\n", app.Name, app.IApplicationAddress.String())
}
