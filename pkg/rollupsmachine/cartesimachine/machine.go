// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package cartesimachine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/pkg/emulator"
)

const (
	AdvanceStateRequest RequestType = 0
	InspectStateRequest RequestType = 1
)

var (
	ErrCartesiMachine = errors.New("cartesi machine internal error")

	ErrTimedOut = errors.New("cartesi machine operation timed out")
	ErrCanceled = errors.New("cartesi machine operation canceled")

	ErrOrphanServer = errors.New("cartesi machine server was left orphan")
)

type cartesiMachine struct {
	server *emulator.RemoteMachine

	address string // address of the JSON RPC remote cartesi machine server
}

// Load loads the machine stored at path into the remote server from address.
func Load(ctx context.Context,
	path string,
	address string,
	config *emulator.MachineRuntimeConfig,
	executionParameters *model.ExecutionParameters,
) (CartesiMachine, error) {
	machine := &cartesiMachine{address: address}

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	jsonConf, err := json.Marshal(config)
	if err != nil {
		err = fmt.Errorf("could not marshal machine runtime config: %w", err)
		return nil, errCartesiMachine(err)
	}

	// Connect to the machine server
	server, err := emulator.ConnectServer(address, executionParameters.FastDeadline)
	if err != nil {
		err = fmt.Errorf("could not connect to the remote machine: %w", err)
		return nil, errCartesiMachine(err)
	}
	machine.server = server

	if err := checkContext(ctx); err != nil {
		server.Delete()
		machine.server = nil
		return nil, err
	}

	// Loads the machine stored at path into the server.
	err = machine.server.Load(path, string(jsonConf))
	if err != nil {
		server.Delete()
		machine.server = nil
		err = fmt.Errorf("could not load the machine: %w", err)
		return nil, errCartesiMachine(err)
	}

	return machine, nil
}

// Fork forks the machine.
//
// When Fork returns with the ErrOrphanServer error, it also returns with a non-nil CartesiMachine
// that can be used to retrieve the orphan server's address.
func (machine *cartesiMachine) Fork(ctx context.Context) (CartesiMachine, error) {
	newMachine := new(cartesiMachine)

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	// Forks the server.
	newServer, address, _, err := machine.server.ForkServer()
	if err != nil {
		err = fmt.Errorf("could not fork the machine: %w", err)
		return nil, errCartesiMachine(err)
	}
	newMachine.address = address
	newMachine.server = newServer

	return newMachine, nil
}

func (machine *cartesiMachine) Run(ctx context.Context, until uint64) (emulator.BreakReason, error) {
	if err := checkContext(ctx); err != nil {
		return -1, err
	}

	breakReason, err := machine.server.Run(until)
	if err != nil {
		assert(breakReason == emulator.BreakReasonFailed, breakReason.String())
		err = fmt.Errorf("machine run failed: %w", err)
		return breakReason, errCartesiMachine(err)
	}
	return breakReason, nil
}

func (machine *cartesiMachine) IsAtManualYield(ctx context.Context) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}

	iflagsY, err := machine.server.ReadReg(emulator.REG_IFLAGS_Y)
	if err != nil {
		err = fmt.Errorf("could not read iflagsY: %w", err)
		return false, errCartesiMachine(err)
	}
	return iflagsY == 1, nil
}

func (machine *cartesiMachine) ReadYieldReason(ctx context.Context) (emulator.CmioYieldReason, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}

	reason, err := machine.server.ReadReg(emulator.REG_HTIF_TOHOST_REASON)
	if err != nil {
		err = fmt.Errorf("could not read HTIF tohost reason: %w", err)
		return 0, errCartesiMachine(err)
	}
	return emulator.CmioYieldReason(reason), nil
}

func (machine *cartesiMachine) ReadHash(ctx context.Context) ([32]byte, error) {
	hash := [32]byte{}
	if err := checkContext(ctx); err != nil {
		return hash, err
	}
	hashSlice, err := machine.server.GetRootHash()
	if err != nil {
		err := fmt.Errorf("could not get the machine's root hash: %w", err)
		return hash, errCartesiMachine(err)
	}
	if len(hashSlice) != 32 {
		err := fmt.Errorf("invalid machine root hash length: expected 32 bytes, got %d bytes", len(hashSlice))
		return hash, errCartesiMachine(err)
	}
	copy(hash[:], hashSlice)
	return hash, nil
}

func (machine *cartesiMachine) ReadMemory(ctx context.Context) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return []byte{}, err
	}

	tohost, err := machine.server.ReadReg(emulator.REG_HTIF_TOHOST_DATA)
	if err != nil {
		err = fmt.Errorf("could not read HTIF tohost data: %w", err)
		return nil, errCartesiMachine(err)
	}
	length := tohost & 0x00000000ffffffff //nolint:mnd

	if err := checkContext(ctx); err != nil {
		return []byte{}, err
	}

	read, err := machine.server.ReadMemory(emulator.CmioTxBufferStart, length)
	if err != nil {
		err := fmt.Errorf("could not read from the memory: %w", err)
		return nil, errCartesiMachine(err)
	}

	return read, nil
}

func (machine *cartesiMachine) WriteRequest(ctx context.Context,
	data []byte,
	type_ RequestType,
) error {
	if err := checkContext(ctx); err != nil {
		return err
	}

	// Writes the request's data.
	err := machine.server.WriteMemory(emulator.CmioRxBufferStart, data)
	if err != nil {
		err := fmt.Errorf("could not write to the memory: %w", err)
		return errCartesiMachine(err)
	}

	if err := checkContext(ctx); err != nil {
		return err
	}

	// Writes the request's type.
	err = machine.server.WriteReg(emulator.REG_HTIF_FROMHOST_REASON, uint64(type_))
	if err != nil {
		err := fmt.Errorf("could not write HTIF fromhost reason: %w", err)
		return errCartesiMachine(err)
	}

	// Writes the request's length.
	err = machine.server.WriteReg(emulator.REG_HTIF_FROMHOST_DATA, uint64(len(data)))
	if err != nil {
		err := fmt.Errorf("could not write HTIF fromhost data: %w", err)
		return errCartesiMachine(err)
	}

	return nil
}

func (machine *cartesiMachine) Continue(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}

	err := machine.server.WriteReg(emulator.REG_IFLAGS_Y, 0)
	if err != nil {
		err = fmt.Errorf("could not reset iflagsY: %w", err)
		return errCartesiMachine(err)
	}
	return nil
}

func (machine *cartesiMachine) ReadCycle(ctx context.Context) (uint64, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}

	cycle, err := machine.server.ReadReg(emulator.REG_MCYCLE)
	if err != nil {
		err = fmt.Errorf("could not read the machine's current cycle: %w", err)
		return cycle, errCartesiMachine(err)
	}
	return cycle, nil
}

func (machine cartesiMachine) PayloadLengthLimit() uint {
	expo := float64(emulator.CmioRxBufferLog2Size)
	var payloadLengthLimit = uint(math.Pow(2, expo)) //nolint:mnd
	return payloadLengthLimit
}

func (machine cartesiMachine) Address() string {
	return machine.address
}

// Close closes the cartesi machine. It also shuts down the remote cartesi machine server.
func (machine *cartesiMachine) Close(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	err := machine.server.ShutdownServer()
	if err != nil {
		err = fmt.Errorf("could not shut down the server: %w", err)
		err = errors.Join(errCartesiMachine(err), errOrphanServerWithAddress(machine.address))
	}
	machine.server = nil
	return err
}

// ------------------------------------------------------------------------------------------------

func errCartesiMachine(err error) error {
	return errors.Join(ErrCartesiMachine, err)
}

func errOrphanServerWithAddress(address string) error {
	return fmt.Errorf("%w at address %s", ErrOrphanServer, address)
}

func checkContext(ctx context.Context) error {
	err := ctx.Err()
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimedOut
	} else if errors.Is(err, context.Canceled) {
		return ErrCanceled
	} else {
		return err
	}
}

func assert(condition bool, s string) {
	if !condition {
		panic("assertion error: " + s)
	}
}
