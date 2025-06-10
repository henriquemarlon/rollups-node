// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

/*
 * Self hosted application is a combo of an application contract + authority contract deployed together
 */
package deploy

import (
	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/pkg/ethutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

func buildSelfhostedDeployment(
	cmd *cobra.Command,
	args []string,
	app *ethutil.ApplicationDeployment,
) (*ethutil.SelfhostedDeployment, error) {
	var selfHostedApplicationFactoryAddress common.Address
	var err error

	if !cmd.Flags().Changed("selfhosted-factory") {
		selfHostedApplicationFactoryAddress, err = config.GetContractsSelfHostedApplicationFactoryAddress()
	} else {
		selfHostedApplicationFactoryAddress, err = parseHexAddress(selfHostedApplicationFactoryAddressParam)
	}
	if err != nil {
		return nil, err
	}

	return &ethutil.SelfhostedDeployment{
		FactoryAddress:   selfHostedApplicationFactoryAddress,
		OwnerAddress:     app.OwnerAddress,
		TemplateHash:     app.TemplateHash,
		DataAvailability: app.DataAvailability,
		EpochLength:      app.EpochLength,
		Salt:             app.Salt,
	}, nil
}
