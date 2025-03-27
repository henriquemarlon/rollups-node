// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// This package is a binding to the emulator's C API.
// Refer to the machine-c files in the emulator's repository for documentation
// (mainly machine-c-api.h and jsonrpc-machine-c-api.h).
package emulator

// #include <stdlib.h>
// #include "cartesi-machine/jsonrpc-machine-c-api.h"
import "C"

import (
	"unsafe"
)

// -----------------------------------------------------------------------------
// RemoteMachine: the remote machine object
// -----------------------------------------------------------------------------

type RemoteMachine struct {
	Machine
}

// set_timeout
func (m *RemoteMachine) SetTimeout(milliseconds int64) error {
	return newError(C.cm_jsonrpc_set_timeout(m.ptr, C.int64_t(milliseconds)))
}

// get_timeout
func (m *RemoteMachine) GetTimeout() (int64, error) {
	var ms C.int64_t
	if err := newError(C.cm_jsonrpc_get_timeout(m.ptr, &ms)); err != nil {
		return 0, err
	}
	return int64(ms), nil
}

// set_cleanup_call
func (m *RemoteMachine) SetCleanupCall(call int32) error {
	// call is one of {CM_JSONRPC_NOTHING, CM_JSONRPC_DESTROY, CM_JSONRPC_SHUTDOWN}
	return newError(C.cm_jsonrpc_set_cleanup_call(m.ptr, C.cm_jsonrpc_cleanup_call(call)))
}

// get_cleanup_call
func (m *RemoteMachine) GetCleanupCall() (int32, error) {
	var call C.cm_jsonrpc_cleanup_call
	if err := newError(C.cm_jsonrpc_get_cleanup_call(m.ptr, &call)); err != nil {
		return 0, err
	}
	return int32(call), nil
}

// get_server_address
func (m *RemoteMachine) GetServerAddress() (string, error) {
	var addr *C.char
	if err := newError(C.cm_jsonrpc_get_server_address(m.ptr, &addr)); err != nil {
		return "", err
	}
	return C.GoString(addr), nil
}

// get_server_version
func (m *RemoteMachine) GetServerVersion() (string, error) {
	var ver *C.char
	if err := newError(C.cm_jsonrpc_get_server_version(m.ptr, &ver)); err != nil {
		return "", err
	}
	return C.GoString(ver), nil
}

// fork_server
func (m *RemoteMachine) ForkServer() (*RemoteMachine, string, uint32, error) {
	var forked *C.cm_machine
	var addr *C.char
	var pid C.uint32_t
	if err := newError(C.cm_jsonrpc_fork_server(m.ptr, &forked, &addr, &pid)); err != nil {
		return nil, "", 0, err
	}
	return &RemoteMachine{Machine: Machine{ptr: forked}}, C.GoString(addr), uint32(pid), nil
}

// rebind_server
func (m *RemoteMachine) RebindServer(newAddr string) (string, error) {
	cAddr := C.CString(newAddr)
	defer C.free(unsafe.Pointer(cAddr))

	var bound *C.char
	if err := newError(C.cm_jsonrpc_rebind_server(m.ptr, cAddr, &bound)); err != nil {
		return "", err
	}
	return C.GoString(bound), nil
}

// shutdown_server
func (m *RemoteMachine) ShutdownServer() error {
	return newError(C.cm_jsonrpc_shutdown_server(m.ptr))
}

// emancipate_server
func (m *RemoteMachine) EmancipateServer() error {
	code := C.cm_jsonrpc_emancipate_server(m.ptr)
	err := newError(code)
	return err
}

// delay_next_request
func (m *RemoteMachine) DelayNextRequest(milliseconds uint64) error {
	return newError(C.cm_jsonrpc_delay_next_request(m.ptr, C.uint64_t(milliseconds)))
}
