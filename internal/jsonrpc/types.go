// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package jsonrpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"regexp"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// -----------------------------------------------------------------------------
// JSON‑RPC message types
// -----------------------------------------------------------------------------

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      any             `json:"id"`
}

type RPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
	ID      any       `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// writeRPCError sends a generic error response for internal errors.
func writeRPCError(w http.ResponseWriter, id any, code int, message string, data any) {
	// Hide detailed error info for internal errors.
	if code == JSONRPC_INTERNAL_ERROR {
		message = "Internal server error"
		data = nil
	}
	resp := RPCResponse{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	resp := RPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// -----------------------------------------------------------------------------
// Parameter and Result types (API)
// -----------------------------------------------------------------------------

var hexAddressRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var appNameRegex = regexp.MustCompile(`^[a-z0-9_-]{3,}$`)

func validateNameOrAddress(nameOrAddress string) error {
	if hexAddressRegex.MatchString(nameOrAddress) {
		_, err := config.ToAddressFromString(nameOrAddress)
		return err
	}
	if appNameRegex.MatchString(nameOrAddress) {
		return nil
	}
	return fmt.Errorf("invalid application name")
}

// Pagination represents common pagination structure used in responses
type Pagination struct {
	TotalCount uint64 `json:"total_count"`
	Limit      uint64 `json:"limit"`
	Offset     uint64 `json:"offset"`
}

// ListApplicationsParams aligns with the OpenRPC specification
type ListApplicationsParams struct {
	Limit  uint64 `json:"limit"`
	Offset uint64 `json:"offset"`
}

// GetApplicationParams aligns with the OpenRPC specification
type GetApplicationParams struct {
	Application string `json:"application"`
}

// ListEpochsParams aligns with the OpenRPC specification
type ListEpochsParams struct {
	Application string  `json:"application"`
	Status      *string `json:"status,omitempty"`
	Limit       uint64  `json:"limit"`
	Offset      uint64  `json:"offset"`
}

// GetEpochParams aligns with the OpenRPC specification
type GetEpochParams struct {
	Application string  `json:"application"`
	EpochIndex  *string `json:"epoch_index"`
}

// ListInputsParams aligns with the OpenRPC specification
type ListInputsParams struct {
	Application string  `json:"application"`
	EpochIndex  *string `json:"epoch_index,omitempty"`
	Sender      *string `json:"sender,omitempty"`
	Limit       uint64  `json:"limit"`
	Offset      uint64  `json:"offset"`
}

// GetInputParams aligns with the OpenRPC specification
type GetInputParams struct {
	Application string  `json:"application"`
	InputIndex  *string `json:"input_index"`
}

// GetProcessedInputCountParams aligns with the OpenRPC specification
type GetProcessedInputCountParams struct {
	Application string `json:"application"`
}

// ListOutputsParams aligns with the OpenRPC specification
type ListOutputsParams struct {
	Application    string  `json:"application"`
	EpochIndex     *string `json:"epoch_index,omitempty"`
	InputIndex     *string `json:"input_index,omitempty"`
	OutputType     *string `json:"output_type,omitempty"`
	VoucherAddress *string `json:"voucher_address,omitempty"`
	Limit          uint64  `json:"limit"`
	Offset         uint64  `json:"offset"`
}

// GetOutputParams aligns with the OpenRPC specification
type GetOutputParams struct {
	Application string  `json:"application"`
	OutputIndex *string `json:"output_index"`
}

// ListReportsParams aligns with the OpenRPC specification
type ListReportsParams struct {
	Application string  `json:"application"`
	EpochIndex  *string `json:"epoch_index,omitempty"`
	InputIndex  *string `json:"input_index,omitempty"`
	Limit       uint64  `json:"limit"`
	Offset      uint64  `json:"offset"`
}

// GetReportParams aligns with the OpenRPC specification
type GetReportParams struct {
	Application string  `json:"application"`
	ReportIndex *string `json:"report_index"`
}

// Result types updated to match the OpenRPC specification
type ApplicationListResult struct {
	Data       []*model.Application `json:"data"`
	Pagination Pagination           `json:"pagination"`
}

type ApplicationGetResult struct {
	Data *model.Application `json:"data"`
}

type ProcessedInputCountResult struct {
	ProcessedInputs uint64 `json:"processed_inputs"`
}

type EpochListResult struct {
	Data       []*model.Epoch `json:"data"`
	Pagination Pagination     `json:"pagination"`
}

type EpochGetResult struct {
	Data *model.Epoch `json:"data"`
}

type InputListResult struct {
	Data       []any      `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type InputGetResult struct {
	Data any `json:"data"`
}

type OutputListResult struct {
	Data       []any      `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type OutputGetResult struct {
	Data any `json:"data"`
}

type ReportListResult struct {
	Data       []*model.Report `json:"data"`
	Pagination Pagination      `json:"pagination"`
}

type ReportGetResult struct {
	Data *model.Report `json:"data"`
}

// -----------------------------------------------------------------------------
// ABI Decoding helpers (provided code)
// -----------------------------------------------------------------------------

type EvmAdvance struct {
	ChainId        string `json:"chain_id"`
	AppContract    string `json:"application_contract"`
	MsgSender      string `json:"sender"`
	BlockNumber    string `json:"block_number"`
	BlockTimestamp string `json:"block_timestamp"`
	PrevRandao     string `json:"prev_randao"`
	Index          string `json:"index"`
	Payload        string `json:"payload"`
}

type DecodedInput struct {
	*model.Input
	DecodedData *EvmAdvance `json:"decoded_data"`
}

func DecodeInput(input *model.Input, parsedAbi *abi.ABI) (*DecodedInput, error) {
	decoded := make(map[string]any)
	err := parsedAbi.Methods["EvmAdvance"].Inputs.UnpackIntoMap(decoded, input.RawData[4:])
	if err != nil {
		return &DecodedInput{Input: input}, err
	}

	evmAdvance := EvmAdvance{
		ChainId:        fmt.Sprintf("0x%x", decoded["chainId"].(*big.Int)),
		AppContract:    decoded["appContract"].(common.Address).Hex(),
		MsgSender:      decoded["msgSender"].(common.Address).Hex(),
		BlockNumber:    fmt.Sprintf("0x%x", decoded["blockNumber"].(*big.Int)),
		BlockTimestamp: fmt.Sprintf("0x%x", decoded["blockTimestamp"].(*big.Int)),
		PrevRandao:     fmt.Sprintf("0x%x", decoded["prevRandao"].(*big.Int)),
		Index:          fmt.Sprintf("0x%x", decoded["index"].(*big.Int)),
		Payload:        "0x" + hex.EncodeToString(decoded["payload"].([]byte)),
	}

	return &DecodedInput{
		Input:       input,
		DecodedData: &evmAdvance,
	}, nil
}

func (d *DecodedInput) MarshalJSON() ([]byte, error) {
	// Marshal the underlying Input using its custom MarshalJSON.
	inputJSON, err := d.Input.MarshalJSON()
	if err != nil {
		return nil, err
	}
	// Ensure inputJSON is a valid JSON object.
	if len(inputJSON) == 0 || inputJSON[len(inputJSON)-1] != '}' {
		return nil, fmt.Errorf("unexpected format from Input.MarshalJSON")
	}

	if d.DecodedData == nil {
		return inputJSON, nil
	}
	// Marshal the DecodedData field.
	decodedDataJSON, err := json.Marshal(d.DecodedData)
	if err != nil {
		return nil, err
	}
	// Use a bytes.Buffer to build the final JSON.
	var buf bytes.Buffer
	buf.Write(bytes.TrimSuffix(inputJSON, []byte("}")))
	buf.WriteString(`,"decoded_data":`)
	buf.Write(decodedDataJSON)
	buf.WriteByte('}')

	return buf.Bytes(), nil
}

type Notice struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type Voucher struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Value       string `json:"value"`
	Payload     string `json:"payload"`
}

type DelegateCallVoucher struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Payload     string `json:"payload"`
}

type DecodedOutput struct {
	*model.Output
	DecodedData any `json:"decoded_data"`
}

func DecodeOutput(output *model.Output, parsedAbi *abi.ABI) (*DecodedOutput, error) {
	decodedOutput := &DecodedOutput{Output: output}
	if len(output.RawData) < 4 {
		return decodedOutput, fmt.Errorf("raw data too short")
	}
	method, err := parsedAbi.MethodById(output.RawData[:4])
	if err != nil {
		return decodedOutput, err
	}
	decoded := make(map[string]any)
	if err := method.Inputs.UnpackIntoMap(decoded, output.RawData[4:]); err != nil {
		return decodedOutput, fmt.Errorf("failed to unpack %s: %w", method.Name, err)
	}
	var result any
	switch method.Name {
	case "Notice":
		payload, ok := decoded["payload"].([]byte)
		if !ok {
			return decodedOutput, fmt.Errorf("unable to decode Notice payload")
		}
		result = Notice{
			Type:    "Notice",
			Payload: "0x" + hex.EncodeToString(payload),
		}
	case "Voucher":
		dest, ok1 := decoded["destination"].(common.Address)
		value, ok2 := decoded["value"].(*big.Int)
		payload, ok3 := decoded["payload"].([]byte)
		if !ok1 || !ok2 || !ok3 {
			return decodedOutput, fmt.Errorf("unable to decode Voucher parameters")
		}
		result = Voucher{
			Type:        "Voucher",
			Destination: dest.Hex(),
			Value:       fmt.Sprintf("0x%x", value),
			Payload:     "0x" + hex.EncodeToString(payload),
		}
	case "DelegateCallVoucher":
		dest, ok1 := decoded["destination"].(common.Address)
		payload, ok2 := decoded["payload"].([]byte)
		if !ok1 || !ok2 {
			return decodedOutput, fmt.Errorf("unable to decode DelegateCallVoucher parameters")
		}
		result = DelegateCallVoucher{
			Type:        "DelegateCallVoucher",
			Destination: dest.Hex(),
			Payload:     "0x" + hex.EncodeToString(payload),
		}
	default:
		result = map[string]any{
			"type":    method.Name,
			"rawData": "0x" + hex.EncodeToString(output.RawData),
		}
	}
	decodedOutput.DecodedData = result
	return decodedOutput, nil
}

func (d *DecodedOutput) MarshalJSON() ([]byte, error) {
	// Marshal the underlying Output using its custom MarshalJSON.
	outputJSON, err := d.Output.MarshalJSON()
	if err != nil {
		return nil, err
	}
	// Ensure outputJSON is a valid JSON object.
	if len(outputJSON) == 0 || outputJSON[len(outputJSON)-1] != '}' {
		return nil, fmt.Errorf("unexpected format from Output.MarshalJSON")
	}

	if d.DecodedData == nil {
		return outputJSON, nil
	}
	// Marshal the DecodedData field.
	decodedDataJSON, err := json.Marshal(d.DecodedData)
	if err != nil {
		return nil, err
	}
	// Use a bytes.Buffer to build the final JSON.
	var buf bytes.Buffer
	buf.Write(bytes.TrimSuffix(outputJSON, []byte("}")))
	buf.WriteString(`,"decoded_data":`)
	buf.Write(decodedDataJSON)
	buf.WriteByte('}')

	return buf.Bytes(), nil
}
