// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func fN(xs []uint64)(func(uint64)(uint64,error)) {
	return func(guess uint64)(uint64,error) {
		for i,x := range xs {
			if (x > guess) {
				return uint64(i), nil
			}
		}
		return uint64(len(xs)), nil
	}
}

// test no values
func TestMBSearch0(t *testing.T) {
	result, err := MBSearch(0, 1, 0, fN([]uint64{}))
	assert.Nil(t, err)
	assert.Equal(t, 0, len(result))
}

// test a single values
func TestMBSearch1(t *testing.T) {
	result, err := MBSearch(0, 4, 1, fN([]uint64{1}))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, uint64(1), result[0])
}

// test "many" values, including repeated ones.
func TestMBSearch4(t *testing.T) {
	result, err := MBSearch(0, 1024, 4, fN([]uint64{1, 100, 100, 1000}))
	assert.Nil(t, err)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, uint64(1), result[0])
	assert.Equal(t, uint64(100), result[1])
	assert.Equal(t, uint64(100), result[2])
	assert.Equal(t, uint64(1000), result[3])
}
