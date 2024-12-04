// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// The config package manages the node configuration, which comes from environment variables.
// The sub-package generate specifies these environment variables.
package config

import (
	"fmt"
	"os"
)

// Auth is used to sign transactions.
type Auth any

// AuthPrivateKey allows signing through private keys.
type AuthPrivateKey struct {
	PrivateKey Redacted[string]
}

// AuthMnemonic allows signing through mnemonics.
type AuthMnemonic struct {
	Mnemonic     Redacted[string]
	AccountIndex Redacted[int]
}

// AuthAWS allows signing through AWS services.
type AuthAWS struct {
	KeyID  Redacted[string]
	Region Redacted[string]
}

// Redacted is a wrapper that redacts a given field from the logs.
type Redacted[T any] struct {
	Value T
}

func (r Redacted[T]) String() string {
	return "[REDACTED]"
}

func AuthFromEnv() Auth {
	switch GetAuthKind() {
	case AuthKindPrivateKeyVar:
		return AuthPrivateKey{
			PrivateKey: Redacted[string]{GetAuthPrivateKey()},
		}
	case AuthKindPrivateKeyFile:
		path := GetAuthPrivateKeyFile()
		privateKey, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("failed to read private-key file: %v", err))
		}
		return AuthPrivateKey{
			PrivateKey: Redacted[string]{string(privateKey)},
		}
	case AuthKindMnemonicVar:
		return AuthMnemonic{
			Mnemonic:     Redacted[string]{GetAuthMnemonic()},
			AccountIndex: Redacted[int]{GetAuthMnemonicAccountIndex()},
		}
	case AuthKindMnemonicFile:
		path := GetAuthMnemonicFile()
		mnemonic, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("failed to read mnemonic file: %v", err))
		}
		return AuthMnemonic{
			Mnemonic:     Redacted[string]{string(mnemonic)},
			AccountIndex: Redacted[int]{GetAuthMnemonicAccountIndex()},
		}
	case AuthKindAWS:
		return AuthAWS{
			KeyID:  Redacted[string]{GetAuthAwsKmsKeyId()},
			Region: Redacted[string]{GetAuthAwsKmsRegion()},
		}
	default:
		panic("invalid auth kind")
	}
}
