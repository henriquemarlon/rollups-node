// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package list

import (
	"encoding/json"
	"fmt"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "list",
	Short:   "Lists all applications",
	Example: examples,
	Run:     run,
}

const examples = `# List all registered applications:
cartesi-rollups-cli app list`

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	applications, _, err := repo.ListApplications(ctx, repository.ApplicationFilter{}, repository.Pagination{})
	cobra.CheckErr(err)

	result, err := json.MarshalIndent(applications, "", "    ")
	cobra.CheckErr(err)

	fmt.Println(string(result))
}
