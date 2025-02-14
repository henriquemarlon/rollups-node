// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package jsonrpc

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
)

//go:embed jsonrpc-discover.json
var discoverSpec embed.FS

const (
	// Maximum allowed body size (1 MB).
	MAX_BODY_SIZE = 1 << 20
	// Maximum amount of items to list (10,000).
	LIST_ITEM_LIMIT = 10000
)

const (
	JSONRPC_PARSE_ERROR      int = -32700
	JSONRPC_INVALID_REQUEST  int = -32600
	JSONRPC_METHOD_NOT_FOUND int = -32601
	JSONRPC_INVALID_PARAMS   int = -32602
	JSONRPC_INTERNAL_ERROR   int = -32603
)

// -----------------------------------------------------------------------------
// Dispatching JSON‑RPC methods
// -----------------------------------------------------------------------------

func (s *Service) handleRPC(w http.ResponseWriter, r *http.Request) {
	// Limit request body size and ensure it is closed.
	r.Body = http.MaxBytesReader(w, r.Body, MAX_BODY_SIZE)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	var req RPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	s.Logger.Info(fmt.Sprintf("Received RPC request: %s", req.Method))
	switch req.Method {
	case "rpc.discover":
		s.handleDiscover(w, r, req)
	case "cartesi_listApplications":
		s.handleListApplications(w, r, req)
	case "cartesi_getApplication":
		s.handleGetApplication(w, r, req)
	case "cartesi_listEpochs":
		s.handleListEpochs(w, r, req)
	case "cartesi_getEpoch":
		s.handleGetEpoch(w, r, req)
	case "cartesi_listInputs":
		s.handleListInputs(w, r, req)
	case "cartesi_getInput":
		s.handleGetInput(w, r, req)
	case "cartesi_getProcessedInputCount":
		s.handleGetProcessedInputCount(w, r, req)
	case "cartesi_listOutputs":
		s.handleListOutputs(w, r, req)
	case "cartesi_getOutput":
		s.handleGetOutput(w, r, req)
	case "cartesi_listReports":
		s.handleListReports(w, r, req)
	case "cartesi_getReport":
		s.handleGetReport(w, r, req)
	default:
		s.Logger.Info(fmt.Sprintf("RPC method not found: %s", req.Method))
		writeRPCError(w, req.ID, JSONRPC_METHOD_NOT_FOUND, "Method not found", nil)
	}
}

// -----------------------------------------------------------------------------
// Individual Method Handlers
// -----------------------------------------------------------------------------

// Discovery: return the embedded specification.
func (s *Service) handleDiscover(w http.ResponseWriter, _ *http.Request, req RPCRequest) {
	data, err := discoverSpec.ReadFile("jsonrpc-discover.json")
	if err != nil {
		s.Logger.Error("Unable to read jsonrpc-discover content", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}
	var spec any
	if err := json.Unmarshal(data, &spec); err != nil {
		s.Logger.Error("Unable to unmarshal discovery spec JSON", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}
	writeRPCResult(w, req.ID, spec)
}

func (s *Service) handleListApplications(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params ListApplicationsParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}
	// Use default values if not provided
	if params.Limit <= 0 {
		params.Limit = 50
	}
	// Cap limit to 10,000.
	if params.Limit > LIST_ITEM_LIMIT {
		params.Limit = LIST_ITEM_LIMIT
	}

	apps, total, err := s.repository.ListApplications(r.Context(), repository.ApplicationFilter{}, repository.Pagination{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		s.Logger.Error("Unable to retrieve applications from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Create result with proper pagination format per spec
	result := struct {
		Data       []*model.Application `json:"data"`
		Pagination struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		} `json:"pagination"`
	}{
		Data: apps,
		Pagination: struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		}{
			TotalCount: total,
			Limit:      params.Limit,
			Offset:     params.Offset,
		},
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleGetApplication(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params GetApplicationParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	app, err := s.repository.GetApplication(r.Context(), params.Application)
	if err != nil {
		s.Logger.Error("Unable to retrieve application from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Return in the format specified in the OpenRPC spec
	result := struct {
		Data *model.Application `json:"data"`
	}{
		Data: app,
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleListEpochs(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params ListEpochsParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Use default values if not provided
	if params.Limit <= 0 {
		params.Limit = 50
	}

	if params.Limit > LIST_ITEM_LIMIT {
		params.Limit = LIST_ITEM_LIMIT
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	var epochFilter repository.EpochFilter
	if params.Status != nil {
		st := model.EpochStatus(*params.Status)
		epochFilter.Status = &st
	}

	epochs, total, err := s.repository.ListEpochs(r.Context(), params.Application, epochFilter, repository.Pagination{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		s.Logger.Error("Unable to retrieve epochs from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Format response according to spec
	result := struct {
		Data       []*model.Epoch `json:"data"`
		Pagination struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		} `json:"pagination"`
	}{
		Data: epochs,
		Pagination: struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		}{
			TotalCount: total,
			Limit:      params.Limit,
			Offset:     params.Offset,
		},
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleGetEpoch(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params GetEpochParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	epoch, err := s.repository.GetEpoch(r.Context(), params.Application, params.Index)
	if err != nil {
		s.Logger.Error("Unable to retrieve epoch from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Format response according to spec
	result := struct {
		Data *model.Epoch `json:"data"`
	}{
		Data: epoch,
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleListInputs(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params ListInputsParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Use default values if not provided
	if params.Limit <= 0 {
		params.Limit = 50
	}

	if params.Limit > LIST_ITEM_LIMIT {
		params.Limit = LIST_ITEM_LIMIT
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	// Create input filter based on params
	inputFilter := repository.InputFilter{}
	if params.EpochIndex != nil {
		epochIndex, err := strconv.ParseUint(*params.EpochIndex, 10, 64)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid epoch index: %v", err), nil)
			return
		}
		inputFilter.EpochIndex = &epochIndex
	}

	// Add sender filter if provided
	if params.Sender != nil {
		sender, err := config.ToAddressFromString(*params.Sender)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid input sender address: %v", err), nil)
			return
		}
		inputFilter.Sender = &sender
	}

	inputs, total, err := s.repository.ListInputs(r.Context(), params.Application, inputFilter, repository.Pagination{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		s.Logger.Error("Unable to retrieve inputs from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	var resultInputs []any
	for _, in := range inputs {
		if params.Decode {
			decoded, err := decodeInput(in, s.inputABI)
			if err != nil {
				s.Logger.Error("Unable to decode Input", "app", params.Application, "index", in.Index, "err", err)
				resultInputs = append(resultInputs, in)
			} else {
				resultInputs = append(resultInputs, decoded)
			}
		} else {
			resultInputs = append(resultInputs, in)
		}
	}

	// Format response according to spec
	result := struct {
		Data       []any `json:"data"`
		Pagination struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		} `json:"pagination"`
	}{
		Data: resultInputs,
		Pagination: struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		}{
			TotalCount: total,
			Limit:      params.Limit,
			Offset:     params.Offset,
		},
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleGetInput(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params GetInputParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	input, err := s.repository.GetInput(r.Context(), params.Application, params.InputIndex)
	if err != nil {
		s.Logger.Error("Unable to retrieve input from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	var result any
	if params.Decode {
		decoded, err := decodeInput(input, s.inputABI)
		if err != nil {
			s.Logger.Error("Unable to decode Input", "app", params.Application, "index", input.Index, "err", err)
			result = input
		} else {
			result = decoded
		}
	} else {
		result = input
	}

	// Format response according to spec
	response := struct {
		Data any `json:"data"`
	}{
		Data: result,
	}

	writeRPCResult(w, req.ID, response)
}

func (s *Service) handleGetProcessedInputCount(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params GetApplicationParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	app, err := s.repository.GetApplication(r.Context(), params.Application)
	if err != nil {
		s.Logger.Error("Unable to retrieve application from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Return processed input count as specified in the spec
	result := struct {
		ProcessedInputs uint64 `json:"processed_inputs"`
	}{
		ProcessedInputs: app.ProcessedInputs,
	}

	writeRPCResult(w, req.ID, result)
}

func parseOutputType(s string) ([]byte, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}
	if len(s) != 8 {
		return []byte{}, fmt.Errorf("invalid output type: expected exactly 4 bytes")
	}
	// Decode the hex string into bytes.
	b, err := hex.DecodeString(s)
	if err != nil {
		return []byte{}, err
	}
	return b, nil
}

func (s *Service) handleListOutputs(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params ListOutputsParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Use default values if not provided
	if params.Limit <= 0 {
		params.Limit = 50
	}

	if params.Limit > LIST_ITEM_LIMIT {
		params.Limit = LIST_ITEM_LIMIT
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	// Create output filter based on params
	outputFilter := repository.OutputFilter{}
	if params.EpochIndex != nil {
		epochIndex, err := strconv.ParseUint(*params.EpochIndex, 10, 64)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid epoch index: %v", err), nil)
			return
		}
		outputFilter.EpochIndex = &epochIndex
	}

	if params.InputIndex != nil {
		inputIndex, err := strconv.ParseUint(*params.InputIndex, 10, 64)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid input index: %v", err), nil)
			return
		}
		outputFilter.InputIndex = &inputIndex
	}

	// Add output type filter if provided
	if params.OutputType != nil {
		outputType, err := parseOutputType(*params.OutputType)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid output type: %v", err), nil)
			return
		}
		outputFilter.OutputType = &outputType
	}

	// Add sender filter if provided
	if params.VoucherAddress != nil {
		voucherAddress, err := config.ToAddressFromString(*params.VoucherAddress)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid voucher address: %v", err), nil)
			return
		}
		outputFilter.VoucherAddress = &voucherAddress
	}

	outputs, total, err := s.repository.ListOutputs(r.Context(), params.Application, outputFilter, repository.Pagination{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		s.Logger.Error("Unable to retrieve outputs from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	var resultOutputs []any
	for _, out := range outputs {
		if params.Decode {
			decoded, err := decodeOutput(out, s.outputABI)
			if err != nil {
				s.Logger.Error("Unable to decode Output", "app", params.Application, "index", out.Index, "err", err)
				resultOutputs = append(resultOutputs, out)
			} else {
				resultOutputs = append(resultOutputs, decoded)
			}
		} else {
			resultOutputs = append(resultOutputs, out)
		}
	}

	// Format response according to spec
	result := struct {
		Data       []any `json:"data"`
		Pagination struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		} `json:"pagination"`
	}{
		Data: resultOutputs,
		Pagination: struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		}{
			TotalCount: total,
			Limit:      params.Limit,
			Offset:     params.Offset,
		},
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleGetOutput(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params GetOutputParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	output, err := s.repository.GetOutput(r.Context(), params.Application, params.OutputIndex)
	if err != nil {
		s.Logger.Error("Unable to retrieve output from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	var result any
	if params.Decode {
		decoded, err := decodeOutput(output, s.outputABI)
		if err != nil {
			s.Logger.Error("Unable to decode Output", "app", params.Application, "index", output.Index, "err", err)
			result = output
		} else {
			result = decoded
		}
	} else {
		result = output
	}

	// Format response according to spec
	response := struct {
		Data any `json:"data"`
	}{
		Data: result,
	}

	writeRPCResult(w, req.ID, response)
}

func (s *Service) handleListReports(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params ListReportsParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Use default values if not provided
	if params.Limit <= 0 {
		params.Limit = 50
	}

	if params.Limit > LIST_ITEM_LIMIT {
		params.Limit = LIST_ITEM_LIMIT
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	// Create report filter based on params
	reportFilter := repository.ReportFilter{}
	if params.EpochIndex != nil {
		epochIndex, err := strconv.ParseUint(*params.EpochIndex, 10, 64)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid epoch index: %v", err), nil)
			return
		}
		reportFilter.EpochIndex = &epochIndex
	}

	if params.InputIndex != nil {
		inputIndex, err := strconv.ParseUint(*params.InputIndex, 10, 64)
		if err != nil {
			writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid input index: %v", err), nil)
			return
		}
		reportFilter.InputIndex = &inputIndex
	}

	reports, total, err := s.repository.ListReports(r.Context(), params.Application, reportFilter, repository.Pagination{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		s.Logger.Error("Unable to retrieve reports from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Format response according to spec
	result := struct {
		Data       []*model.Report `json:"data"`
		Pagination struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		} `json:"pagination"`
	}{
		Data: reports,
		Pagination: struct {
			TotalCount uint64 `json:"total_count"`
			Limit      uint64 `json:"limit"`
			Offset     uint64 `json:"offset"`
		}{
			TotalCount: total,
			Limit:      params.Limit,
			Offset:     params.Offset,
		},
	}

	writeRPCResult(w, req.ID, result)
}

func (s *Service) handleGetReport(w http.ResponseWriter, r *http.Request, req RPCRequest) {
	var params GetReportParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, "Invalid params", nil)
		return
	}

	// Validate application parameter
	if err := validateNameOrAddress(params.Application); err != nil {
		writeRPCError(w, req.ID, JSONRPC_INVALID_PARAMS, fmt.Sprintf("Invalid application identifier: %v", err), nil)
		return
	}

	report, err := s.repository.GetReport(r.Context(), params.Application, params.ReportIndex)
	if err != nil {
		s.Logger.Error("Unable to retrieve report from repository", "err", err)
		writeRPCError(w, req.ID, JSONRPC_INTERNAL_ERROR, "Internal server error", nil)
		return
	}

	// Format response according to spec
	response := struct {
		Data *model.Report `json:"data"`
	}{
		Data: report,
	}

	writeRPCResult(w, req.ID, response)
}
