// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package status

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository/factory"
)

var Cmd = &cobra.Command{
	Use:     "status [app-name-or-address] [new-status]",
	Short:   "Display or set application status (enabled or disabled)",
	Example: examples,
	Args:    cobra.RangeArgs(1, 2), // nolint: mnd
	Run:     run,
}

const examples = `# Get application status:
cartesi-rollups-cli app status echo-dapp

# Set application status:
cartesi-rollups-cli app status echo-dapp enabled
cartesi-rollups-cli app status echo-dapp disabled`

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

	// If no new status is provided, just display the current status
	if len(args) == 1 {
		fmt.Println(app.State)
		os.Exit(0)
	}

	// Handle status change
	newStatus := strings.ToLower(args[1])

	if app.State == model.ApplicationState_Inoperable {
		fmt.Fprintf(os.Stderr, "Error: Cannot execute operation. Application %s is in %s state\n", app.Name, app.State)
		os.Exit(1)
	}

	var targetState model.ApplicationState
	switch newStatus {
	case "enabled", "enable":
		targetState = model.ApplicationState_Enabled
	case "disabled", "disable":
		targetState = model.ApplicationState_Disabled
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid status %q. Valid values are 'enabled' or 'disabled'\n", newStatus)
		os.Exit(1)
	}

	if app.State == targetState {
		fmt.Printf("Application %s status is already %s\n", app.Name, app.State)
		os.Exit(0)
	}

	err = repo.UpdateApplicationState(ctx, app.ID, targetState, nil)
	cobra.CheckErr(err)

	fmt.Printf("Application %s status updated to %s\n", app.Name, targetState)
}
