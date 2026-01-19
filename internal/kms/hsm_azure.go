//go:build azure
// +build azure

package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

// AzureKeyVaultProvider implements HSMProvider using Azure Key Vault.
type AzureKeyVaultProvider struct {
	client   *azkeys.Client
	keyName  string
	key      []byte
	aead     cipher.AEAD
	ctx      context.Context
}

// NewAzureKeyVaultProvider creates a new Azure Key Vault provider.
//
// Parameters:
//   - vaultURL: Azure Key Vault URL (e.g., "https://myvault.vault.azure.net/")
//   - keyName: Name of the key in Key Vault
func NewAzureKeyVaultProvider(vaultURL, keyName string) (*AzureKeyVaultProvider, error) {
	ctx := context.Background()

	// Use DefaultAzureCredential (supports multiple auth methods)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Key Vault client: %w", err)
	}

	// Get the key from Key Vault
	keyResp, err := client.GetKey(ctx, keyName, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get key from Azure Key Vault: %w", err)
	}

	// Extract key material (if available)
	var key []byte
	if keyResp.Key != nil && keyResp.Key.N != nil {
		// RSA key - not suitable for AES, would need to use RSA encryption
		return nil, errors.New("RSA keys not supported, use AES keys")
	}

	// For AES keys, Azure Key Vault doesn't export key material directly
	// We'll use Azure Key Vault's encrypt/decrypt operations
	// For now, return error to indicate envelope encryption should be used
	return nil, errors.New("Azure Key Vault requires using encrypt/decrypt operations directly")

	// Note: Full implementation would use client.Encrypt() and client.Decrypt()
	// but those require different key types and operations
}

func (a *AzureKeyVaultProvider) GetKey(keyID string) ([]byte, error) {
	return a.key, nil
}

func (a *AzureKeyVaultProvider) Encrypt(keyID string, plaintext []byte) (ciphertext, nonce []byte, err error) {
	if a.aead == nil {
		return nil, nil, errors.New("Azure Key Vault provider not properly initialized")
	}

	nonce = make([]byte, a.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = a.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func (a *AzureKeyVaultProvider) Decrypt(keyID string, ciphertext, nonce []byte) ([]byte, error) {
	if a.aead == nil {
		return nil, errors.New("Azure Key Vault provider not properly initialized")
	}

	plaintext, err := a.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func (a *AzureKeyVaultProvider) Close() error {
	if a.key != nil {
		for i := range a.key {
			a.key[i] = 0
		}
		a.key = nil
	}
	return nil
}

