// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package inspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cartesi/rollups-node/internal/advancer/machines"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	ErrInvalidMachines = errors.New("machines must not be nil")
	ErrNoApp           = errors.New("no machine for application")
)

type Inspector struct {
	IInspectMachines
	Logger   *slog.Logger
	ServeMux *http.ServeMux
}

type ReportResponse struct {
	Payload string `json:"payload"`
}

type InspectResponse struct {
	Status          string           `json:"status"`
	Exception       string           `json:"exception"`
	Reports         []ReportResponse `json:"reports"`
	ProcessedInputs uint64           `json:"processed_input_count"`
}

func (s *Inspector) CreateInspectServer(
	addr string,
	maxRetries int,
	retryInterval time.Duration,
	mux *http.ServeMux,
) (*http.Server, func() error) {
	server := &http.Server{
		Addr:     addr,
		Handler:  mux,
		ErrorLog: slog.NewLogLogger(s.Logger.Handler(), slog.LevelError),
	}
	return server, func() error {
		s.Logger.Info("Create Inspect Server", "addr", addr)
		var err error = nil
		for retry := 0; retry < maxRetries+1; retry++ {
			switch err = server.ListenAndServe(); err {
			case http.ErrServerClosed:
				return nil
			default:
				s.Logger.Error("http",
					"error", err,
					"try", retry+1,
					"maxRetries", maxRetries,
					"error", err)
			}
			time.Sleep(retryInterval)
		}
		return err
	}
}

func (inspect *Inspector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		dapp         string
		payload      []byte
		err          error
		reports      []ReportResponse
		status       string
		errorMessage string
	)

	if r.PathValue("dapp") == "" {
		inspect.Logger.Info("Bad request",
			"err", "Missing application address")
		http.Error(w, "Missing application address", http.StatusBadRequest)
		return
	}

	dapp = strings.ToLower(common.HexToAddress(r.PathValue("dapp")).String())
	if r.Method == "POST" {
		payload, err = io.ReadAll(r.Body)
		if err != nil {
			inspect.Logger.Info("Bad request",
				"err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if r.PathValue("payload") == "" {
			inspect.Logger.Info("Bad request",
				"err", "Missing payload")
			http.Error(w, "Missing payload", http.StatusBadRequest)
			return
		}
		decodedValue, err := url.PathUnescape(r.PathValue("payload"))
		if err != nil {
			inspect.Logger.Error("Internal server error",
				"err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload = []byte(decodedValue)
	}

	inspect.Logger.Info("Got new inspect request", "application", dapp)
	result, err := inspect.process(r.Context(), dapp, payload)
	if err != nil {
		if errors.Is(err, ErrNoApp) {
			inspect.Logger.Error("Application not found", "address", dapp, "err", err)
			http.Error(w, "Application not found", http.StatusNotFound)
			return
		}
		inspect.Logger.Error("Internal server error", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, report := range result.Reports {
		reports = append(reports, ReportResponse{Payload: hexutil.Encode(report)})
	}

	if result.Accepted {
		status = "Accepted"
	} else {
		status = "Rejected"
	}

	if result.Error != nil {
		status = "Exception"
		errorMessage = fmt.Sprintf("Error on the machine while inspecting: %s", result.Error)
	}

	response := InspectResponse{
		Status:          status,
		Exception:       errorMessage,
		Reports:         reports,
		ProcessedInputs: result.ProcessedInputs,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		inspect.Logger.Error("Internal server error",
			"err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	inspect.Logger.Info("Request executed",
		"status", status,
		"application", dapp)
}

// process sends an inspect request to the machine
func (inspect *Inspector) process(
	ctx context.Context,
	app string,
	query []byte) (*InspectResult, error) {
	// Asserts that the app has an associated machine.
	machine, exists := inspect.GetInspectMachine(app)
	if !exists {
		return nil, fmt.Errorf("%w %s", ErrNoApp, app)
	}

	res, err := machine.Inspect(ctx, query)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// ------------------------------------------------------------------------------------------------

type IInspectMachines interface {
	GetInspectMachine(app string) (machines.InspectMachine, bool)
}

type IInspectMachine interface {
	Inspect(_ context.Context, query []byte) (*InspectResult, error)
}
