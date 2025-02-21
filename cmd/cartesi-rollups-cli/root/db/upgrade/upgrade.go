// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package upgrade

import (
	"fmt"
	"os"
	"time"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/repository/postgres/schema"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Create or Upgrade the Database Schema",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	var s *schema.Schema
	var err error

	dsnURL, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	for i := 0; i < 5; i++ {
		s, err = schema.New(dsnURL.String())
		if err == nil {
			break
		}
		if i == 4 {
			fmt.Fprintf(os.Stderr, "Failed to connect to database. (%s)\n", dsnURL.Redacted())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Connection to database failed. Trying again... (%s)\n", dsnURL.Redacted())
		time.Sleep(5 * time.Second) // wait before retrying
	}
	defer s.Close()

	err = s.Upgrade()
	cobra.CheckErr(err)

	version, err := s.ValidateVersion()
	cobra.CheckErr(err)

	fmt.Printf("Database Schema was successfully upgraded. Current version: %d\n", version)
}
