// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package check

import (
	"fmt"
	"os"
	"time"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/repository/postgres/schema"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "check-version",
	Short: "Validate the Database Schema version",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	dsnURL, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	var s *schema.Schema
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

	version, err := s.ValidateVersion()
	cobra.CheckErr(err)

	fmt.Printf("Database Schema is at the correct version: %d", version)
}
