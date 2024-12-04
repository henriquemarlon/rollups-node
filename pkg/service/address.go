// Implementation of the pflags Value interface.
package service

import (
	"github.com/ethereum/go-ethereum/common"
)

type EthAddress common.Address

func (me EthAddress) String() string {
	return common.Address(me).String()
}
func (me *EthAddress) Set(s string) error {
	return (*common.Address)(me).UnmarshalText([]byte(s))
}
func (me *EthAddress) Type() string {
	return "EthAddress"
}
