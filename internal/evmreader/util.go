// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"cmp"
	"fmt"
	"slices"

	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/ethereum/go-ethereum/common"
)

// calculateEpochIndex calculates the epoch index given the input block number
// and epoch length
func calculateEpochIndex(epochLength uint64, blockNumber uint64) uint64 {
	return blockNumber / epochLength
}

// appsToAddresses
func appsToAddresses(apps []appContracts) []common.Address {
	var addresses []common.Address
	for _, app := range apps {
		addresses = append(addresses, app.application.IApplicationAddress)
	}
	return addresses
}

func mapAddressToApp(apps []appContracts) map[common.Address]appContracts {
	result := make(map[common.Address]appContracts)
	for _, app := range apps {
		result[app.application.IApplicationAddress] = app
	}
	return result
}

// sortByInputIndex is a compare function that orders Inputs
// by index field. It is intended to be used with `insertSorted`, see insertSorted()
func sortByInputIndex(a, b *Input) int {
	return cmp.Compare(a.Index, b.Index)
}

// insertSorted inserts the received input in the slice at the position defined
// by its index property.
func insertSorted[T any](compare func(a, b *T) int, slice []*T, item *T) []*T {
	// Insert Sorted
	i, _ := slices.BinarySearchFunc(
		slice,
		item,
		compare)
	return slices.Insert(slice, i, item)
}

// Index applications given a key extractor function
func indexApps[K comparable](
	keyExtractor func(appContracts) K,
	apps []appContracts,
) map[K][]appContracts {

	result := make(map[K][]appContracts)
	for _, item := range apps {
		key := keyExtractor(item)
		result[key] = append(result[key], item)
	}
	return result
}

// MBSearch is a multiple binary search over the function f.
// It will find zero, one or multiple transition points x such that f(x-1) < f(x).
// In addition, it will narrow the search space of subsequent points while probing f.
// NOTE: This function assumes that f(0) == 0. In other words: that the transition
// from 0 to 1 exists in the function image.
func MBSearch(minBlock uint64, maxBlock, entries uint64, f func(uint64) (uint64, error)) ([]uint64, error) {
	if entries == 0 {
		return nil, nil
	}

	low := make([]uint64, entries+1)
	high := make([]uint64, entries+1)

	for i := range entries + 1 {
		low[i] = minBlock
		high[i] = maxBlock
	}

	for end := entries + 1; end > 1; {
		guess := (high[end-1] + low[end-1]) / 2
		index, err := f(guess)

		if err != nil {
			return nil, fmt.Errorf("call failed with index %v: %w", guess, err)
		}

		for i := uint64(1); i < index+1; i++ {
			if high[i] > guess {
				high[i] = guess
			}
		}

		for i := index + 1; i < end; i++ {
			if low[i] < guess {
				low[i] = guess
			}
		}

		if low[end-1]+1 == high[end-1] {
			end--
		}
	}
	return high[1:], nil // discard the 0 entry.
}
