//go:build !azure
// +build !azure

package kms

import "errors"

// Stub when azure build tag is not set.
func NewAzureKeyVaultProvider(vaultURL, keyName string) (HSMProvider, error) {
	return nil, errors.New("Azure Key Vault support not compiled (use build tag: azure)")
}


