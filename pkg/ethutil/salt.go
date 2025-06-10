// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package ethutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// create a type for salt to print correctly as JSON
type SaltBytes [32]byte

func ParseSalt(salt string) (SaltBytes, error) {
	data, err := hex.DecodeString(TrimHex(salt))
	if err != nil {
		return [32]byte{}, err
	}
	if len(data) != 32 {
		return [32]byte{}, fmt.Errorf("invalid salt length(%d) != 32", len(data))
	}
	var out [32]byte
	copy(out[:], data)
	return out, nil
}

// print it as a single string instead of multiple integer values
func (me *SaltBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(me[:]))
}
