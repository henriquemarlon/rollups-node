// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// This package is a binding to the emulator's C API.
// Refer to the machine-c files in the emulator's repository for documentation
// (mainly machine-c-api.h and jsonrpc-machine-c-api.h).
package emulator

// #include <stdlib.h>
// #include "cartesi-machine/machine-c-api.h"
import "C"

import (
	"unsafe"
)

// -----------------------------------------------------------------------------
// Machine Methods
// -----------------------------------------------------------------------------

type Machine struct {
	ptr *C.cm_machine
}

// destroy
func (m *Machine) Destroy() error {
	return newError(C.cm_destroy(m.ptr))
}

// delete
func (m *Machine) Delete() {
	C.cm_delete(m.ptr)
}

// create
func (m *Machine) Create(config, runtimeConfig string) error {
	var cConfig *C.char
	var cRuntime *C.char
	if config != "" {
		cConfig = C.CString(config)
		defer C.free(unsafe.Pointer(cConfig))
	}
	if runtimeConfig != "" {
		cRuntime = C.CString(runtimeConfig)
		defer C.free(unsafe.Pointer(cRuntime))
	}
	return newError(C.cm_create_new(cConfig, cRuntime, &m.ptr))
}

// get_default_config
func (m *Machine) GetDefaultConfig() (string, error) {
	var cfg *C.char
	// passing 'm->ptr' so it can fetch remote defaults if the API supports that
	if err := newError(C.cm_get_default_config(m.ptr, &cfg)); err != nil {
		return "", err
	}
	if cfg == nil {
		return "", nil
	}
	res := C.GoString(cfg)
	C.free(unsafe.Pointer(cfg))
	return res, nil
}

// get_initial_config
func (m *Machine) GetInitialConfig() (string, error) {
	var cfg *C.char
	if err := newError(C.cm_get_initial_config(m.ptr, &cfg)); err != nil {
		return "", err
	}
	if cfg == nil {
		return "", nil
	}
	res := C.GoString(cfg)
	C.free(unsafe.Pointer(cfg))
	return res, nil
}

// get_memory_ranges
func (m *Machine) GetMemoryRanges() (string, error) {
	var ranges *C.char
	if err := newError(C.cm_get_memory_ranges(m.ptr, &ranges)); err != nil {
		return "", err
	}
	if ranges == nil {
		return "", nil
	}
	res := C.GoString(ranges)
	C.free(unsafe.Pointer(ranges))
	return res, nil
}

// get_proof
func (m *Machine) GetProof(address uint64, log2size int32) (string, error) {
	var proof *C.char
	if err := newError(C.cm_get_proof(m.ptr, C.uint64_t(address), C.int32_t(log2size), &proof)); err != nil {
		return "", err
	}
	if proof == nil {
		return "", nil
	}
	res := C.GoString(proof)
	C.free(unsafe.Pointer(proof))
	return res, nil
}

// get_reg_address
func (m *Machine) GetRegAddress(reg RegID) (uint64, error) {
	var val C.uint64_t
	if err := newError(C.cm_get_reg_address(m.ptr, C.cm_reg(reg), &val)); err != nil {
		return 0, err
	}
	return uint64(val), nil
}

// get_root_hash
func (m *Machine) GetRootHash() ([]byte, error) {
	var hash C.cm_hash
	if err := newError(C.cm_get_root_hash(m.ptr, &hash)); err != nil {
		return nil, err
	}
	return C.GoBytes(unsafe.Pointer(&hash), 32), nil
}

// get_runtime_config
func (m *Machine) GetRuntimeConfig() (string, error) {
	var rcfg *C.char
	if err := newError(C.cm_get_runtime_config(m.ptr, &rcfg)); err != nil {
		return "", err
	}
	if rcfg == nil {
		return "", nil
	}
	str := C.GoString(rcfg)
	C.free(unsafe.Pointer(rcfg))
	return str, nil
}

// is_empty
func (m *Machine) IsEmpty() (bool, error) {
	var yes C.bool
	if err := newError(C.cm_is_empty(m.ptr, &yes)); err != nil {
		return false, err
	}
	return bool(yes), nil
}

// load
func (m *Machine) Load(dir string, runtimeConfig string) error {
	cDir := C.CString(dir)
	defer C.free(unsafe.Pointer(cDir))

	var cRuntime *C.char
	if runtimeConfig != "" {
		cRuntime = C.CString(runtimeConfig)
		defer C.free(unsafe.Pointer(cRuntime))
	}
	return newError(C.cm_load(m.ptr, cDir, cRuntime))
}

// read_memory
func (m *Machine) ReadMemory(address, length uint64) ([]byte, error) {
	if length == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, length)
	if err := newError(
		C.cm_read_memory(m.ptr, C.uint64_t(address),
			(*C.uchar)(unsafe.Pointer(&buf[0])),
			C.uint64_t(length)),
	); err != nil {
		return nil, err
	}
	return buf, nil
}

// write_memory
func (m *Machine) WriteMemory(address uint64, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	code := C.cm_write_memory(m.ptr,
		C.uint64_t(address),
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.uint64_t(len(data)))
	return newError(code)
}

// read_reg
func (m *Machine) ReadReg(r RegID) (uint64, error) {
	var val C.uint64_t
	if err := newError(C.cm_read_reg(m.ptr, C.cm_reg(r), &val)); err != nil {
		return 0, err
	}
	return uint64(val), nil
}

// read_reg
func (m *Machine) WriteReg(r RegID, value uint64) error {
	return newError(C.cm_write_reg(m.ptr, C.cm_reg(r), C.uint64_t(value)))
}

// receive_cmio_request
func (m *Machine) ReceiveCmioRequest() (cmd uint8, reason uint16, data []byte, err error) {
	var cCmd C.uint8_t
	var cReason C.uint16_t

	// We'll guess some large buffer.
	// If the new API pattern is different, adapt as needed.
	maxBuf := 2 * 1024 * 1024
	buf := make([]byte, maxBuf)
	var cLen C.uint64_t = C.uint64_t(maxBuf)

	e := newError(C.cm_receive_cmio_request(
		m.ptr,
		&cCmd,
		&cReason,
		(*C.uint8_t)(unsafe.Pointer(&buf[0])),
		&cLen,
	))
	if e != nil {
		return 0, 0, nil, e
	}
	return uint8(cCmd), uint16(cReason), buf[:cLen], nil
}

// run
func (m *Machine) Run(mcycleEnd uint64) (BreakReason, error) {
	var br C.cm_break_reason
	if err := newError(C.cm_run(m.ptr, C.uint64_t(mcycleEnd), &br)); err != nil {
		return BreakReasonFailed, err
	}
	return BreakReason(br), nil
}

// send_cmio_response
func (m *Machine) SendCmioResponse(reason uint16, data []byte) error {
	var ptrData *C.uint8_t
	sizeData := C.uint64_t(len(data))
	if sizeData > 0 {
		ptrData = (*C.uint8_t)(unsafe.Pointer(&data[0]))
	}

	return newError(C.cm_send_cmio_response(
		m.ptr,
		C.uint16_t(reason),
		ptrData,
		sizeData,
	))
}

// set_runtime_config
func (m *Machine) SetRuntimeConfig(runtimeJSON string) error {
	var rc *C.char
	if runtimeJSON != "" {
		rc = C.CString(runtimeJSON)
		defer C.free(unsafe.Pointer(rc))
	}
	return newError(C.cm_set_runtime_config(m.ptr, rc))
}

// store
func (m *Machine) Store(directory string) error {
	cDir := C.CString(directory)
	defer C.free(unsafe.Pointer(cDir))
	return newError(C.cm_store(m.ptr, cDir))
}
