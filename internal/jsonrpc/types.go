// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package jsonrpc

import (
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
	ID      interface{}     `json:"id"`
}

type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeRPCError sends a generic error response for internal errors.
func writeRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
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

func writeRPCResult(w http.ResponseWriter, id interface{}, result interface{}) {
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
	Application string `json:"application"`
	Index       uint64 `json:"index"`
}

// ListInputsParams aligns with the OpenRPC specification
type ListInputsParams struct {
	Application string  `json:"application"`
	EpochIndex  *string `json:"epoch_index,omitempty"`
	Sender      *string `json:"sender,omitempty"`
	Limit       uint64  `json:"limit"`
	Offset      uint64  `json:"offset"`
	Decode      bool    `json:"decode,omitempty"`
}

// GetInputParams aligns with the OpenRPC specification
type GetInputParams struct {
	Application string `json:"application"`
	InputIndex  uint64 `json:"input_index"`
	Decode      bool   `json:"decode,omitempty"`
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
	Decode         bool    `json:"decode,omitempty"`
}

// GetOutputParams aligns with the OpenRPC specification
type GetOutputParams struct {
	Application string `json:"application"`
	OutputIndex uint64 `json:"output_index"`
	Decode      bool   `json:"decode,omitempty"`
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
	Application string `json:"application"`
	ReportIndex uint64 `json:"report_index"`
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
	Data       []interface{} `json:"data"`
	Pagination Pagination    `json:"pagination"`
}

type InputGetResult struct {
	Data interface{} `json:"data"`
}

type OutputListResult struct {
	Data       []interface{} `json:"data"`
	Pagination Pagination    `json:"pagination"`
}

type OutputGetResult struct {
	Data interface{} `json:"data"`
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
	ChainId        string `json:"chainId"`
	AppContract    string `json:"appContract"`
	MsgSender      string `json:"msgSender"`
	BlockNumber    string `json:"blockNumber"`
	BlockTimestamp string `json:"blockTimestamp"`
	PrevRandao     string `json:"prevRandao"`
	Index          string `json:"index"`
	Payload        string `json:"payload"`
}

type DecodedInput struct {
	model.Input
	DecodedData EvmAdvance `json:"decoded_data"`
}

func decodeInput(input *model.Input, parsedAbi *abi.ABI) (*DecodedInput, error) {
	decoded := make(map[string]interface{})
	err := parsedAbi.Methods["EvmAdvance"].Inputs.UnpackIntoMap(decoded, input.RawData[4:])
	if err != nil {
		return nil, err
	}

	evmAdvance := EvmAdvance{
		ChainId:        decoded["chainId"].(*big.Int).String(),
		AppContract:    decoded["appContract"].(common.Address).Hex(),
		MsgSender:      decoded["msgSender"].(common.Address).Hex(),
		BlockNumber:    decoded["blockNumber"].(*big.Int).String(),
		BlockTimestamp: decoded["blockTimestamp"].(*big.Int).String(),
		PrevRandao:     decoded["prevRandao"].(*big.Int).String(),
		Index:          decoded["index"].(*big.Int).String(),
		Payload:        "0x" + hex.EncodeToString(decoded["payload"].([]byte)),
	}

	return &DecodedInput{
		Input:       *input,
		DecodedData: evmAdvance,
	}, nil
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
	model.Output
	DecodedData any `json:"decoded_data"`
}

func decodeOutput(output *model.Output, parsedAbi *abi.ABI) (*DecodedOutput, error) {
	if len(output.RawData) < 4 {
		return nil, fmt.Errorf("raw data too short")
	}
	method, err := parsedAbi.MethodById(output.RawData[:4])
	if err != nil {
		return nil, err
	}
	decoded := make(map[string]interface{})
	if err := method.Inputs.UnpackIntoMap(decoded, output.RawData[4:]); err != nil {
		return nil, fmt.Errorf("failed to unpack %s: %w", method.Name, err)
	}
	var result any
	switch method.Name {
	case "Notice":
		payload, ok := decoded["payload"].([]byte)
		if !ok {
			return nil, fmt.Errorf("unable to decode Notice payload")
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
			return nil, fmt.Errorf("unable to decode Voucher parameters")
		}
		result = Voucher{
			Type:        "Voucher",
			Destination: dest.Hex(),
			Value:       value.String(),
			Payload:     "0x" + hex.EncodeToString(payload),
		}
	case "DelegateCallVoucher":
		dest, ok1 := decoded["destination"].(common.Address)
		payload, ok2 := decoded["payload"].([]byte)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("unable to decode DelegateCallVoucher parameters")
		}
		result = DelegateCallVoucher{
			Type:        "DelegateCallVoucher",
			Destination: dest.Hex(),
			Payload:     "0x" + hex.EncodeToString(payload),
		}
	default:
		result = map[string]interface{}{
			"type":    method.Name,
			"rawData": "0x" + hex.EncodeToString(output.RawData),
		}
	}
	return &DecodedOutput{
		Output:      *output,
		DecodedData: result,
	}, nil
}
