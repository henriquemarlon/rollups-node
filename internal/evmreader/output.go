// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"bytes"
	"context"

	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

func (r *Service) checkForOutputExecution(
	ctx context.Context,
	apps []appContracts,
	mostRecentBlockNumber uint64,
) {

	appAddresses := appsToAddresses(apps)

	r.Logger.Debug("Checking for new Output Executed Events", "apps", appAddresses)

	for _, app := range apps {

		// Safeguard: Only check blocks starting from the block where the InputBox
		// contract was deployed as Inputs can be added to that same block
		lastOutputCheck := max(app.application.LastOutputCheckBlock, app.application.IInputBoxBlock)

		if mostRecentBlockNumber > lastOutputCheck {

			r.Logger.Debug("Checking output execution for application",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"last_output_check block", lastOutputCheck,
				"most_recent_block", mostRecentBlockNumber)

			r.readAndUpdateOutputs(ctx, app, lastOutputCheck, mostRecentBlockNumber)

		} else if mostRecentBlockNumber < lastOutputCheck {
			r.Logger.Warn(
				"Not reading output execution: most recent block is lower than the last processed one",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"last_output_check_block", lastOutputCheck,
				"most_recent_block", mostRecentBlockNumber,
			)
		} else {
			r.Logger.Warn("Not reading output execution: already checked the most recent blocks",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"last output check block", lastOutputCheck,
				"most recent block", mostRecentBlockNumber,
			)
		}
	}

}

func (r *Service) readAndUpdateOutputs(
	ctx context.Context, app appContracts, lastOutputCheck, mostRecentBlockNumber uint64) {

	contract := app.applicationContract

	opts := &bind.FilterOpts{
		Context: ctx,
		Start:   lastOutputCheck + 1,
		End:     &mostRecentBlockNumber,
	}

	outputExecutedEvents, err := contract.RetrieveOutputExecutionEvents(opts)
	if err != nil {
		r.Logger.Error("Error reading output events",
			"application", app.application.Name, "address", app.application.IApplicationAddress,
			"error", err)
		return
	}

	if len(outputExecutedEvents) == 0 {
		r.Logger.Debug("No output executed events found",
			"application", app.application.Name, "address", app.application.IApplicationAddress)
		err := r.repository.UpdateEventLastCheckBlock(
			ctx, []int64{app.application.ID}, MonitoredEvent_OutputExecuted, mostRecentBlockNumber)
		if err != nil {
			r.Logger.Error("Failed to update LastOutputCheckBlock for applications without inputs",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"block_number", mostRecentBlockNumber,
				"error", err,
			)
			// We don't return an error here as there is no output execution to process
			// and this is just an update to the last check block
		} else {
			r.Logger.Debug("Updated LastOutputCheckBlock for applications without inputs",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"block_number", mostRecentBlockNumber,
			)
		}
		return
	}

	// Should we check the output hash??
	var executedOutputs []*Output
	for _, event := range outputExecutedEvents {

		// Compare output to check it is the correct one
		output, err := r.repository.GetOutput(ctx, app.application.IApplicationAddress.Hex(), event.OutputIndex)
		if err != nil {
			r.Logger.Error("Error retrieving output",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"index", event.OutputIndex,
				"error", err)
			return
		}

		if output == nil {
			r.Logger.Warn("Found OutputExecuted event but output does not exist in the database yet",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"index", event.OutputIndex)
			return
		}

		if !bytes.Equal(output.RawData, event.Output) {
			r.Logger.Debug("Output mismatch",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"index", event.OutputIndex,
				"actual", output.RawData,
				"event's", event.Output)

			r.Logger.Error("Output mismatch. Application is in an invalid state",
				"application", app.application.Name, "address", app.application.IApplicationAddress,
				"index", event.OutputIndex)

			return
		}

		r.Logger.Info("Output executed",
			"application", app.application.Name, "address", app.application.IApplicationAddress,
			"index", event.OutputIndex)
		output.ExecutionTransactionHash = &event.Raw.TxHash
		executedOutputs = append(executedOutputs, output)
	}

	err = r.repository.UpdateOutputsExecution(
		ctx, app.application.IApplicationAddress.Hex(), executedOutputs, mostRecentBlockNumber)
	if err != nil {
		r.Logger.Error("Error storing output execution statuses",
			"application", app.application.Name, "address", app.application.IApplicationAddress,
			"error", err)
	}

}
