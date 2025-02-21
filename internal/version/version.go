// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package version

var (
	// Should be overridden during the final release build with ldflags
	// to contain the actual version number
	BuildVersion = "devel"
)
