package kms

import (
	"encoding/base64"
	"fmt"
)

const (
	// AESGCMNonceSize is the standard nonce size for AES-GCM encryption (12 bytes)
	AESGCMNonceSize = 12
)

// CombineNonceAndCiphertext combines nonce and ciphertext into a single base64 string.
// Format: base64(nonce + ciphertext)
// This is useful for storing encrypted data in a single database field instead of separate BLOB columns.
//
// Example usage:
//   encrypted := CombineNonceAndCiphertext(nonce, ciphertext)
//   // Store encrypted in database as VARCHAR/TEXT
//
// To decrypt:
//   nonce, ciphertext, err := SplitNonceAndCiphertext(encrypted, AESGCMNonceSize)
func CombineNonceAndCiphertext(nonce, ciphertext []byte) string {
	combined := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined)
}

// SplitNonceAndCiphertext splits a base64 string back into nonce and ciphertext.
// The nonceSize parameter should be AESGCMNonceSize (12 bytes) for AES-GCM encryption.
//
// Returns:
//   - nonce: The initialization vector used for encryption
//   - ciphertext: The encrypted data
//   - error: Any error that occurred during decoding or splitting
func SplitNonceAndCiphertext(combinedBase64 string, nonceSize int) (nonce, ciphertext []byte, err error) {
	combined, err := base64.StdEncoding.DecodeString(combinedBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	if len(combined) < nonceSize {
		return nil, nil, fmt.Errorf("combined data too short: expected at least %d bytes, got %d", nonceSize, len(combined))
	}
	nonce = combined[:nonceSize]
	ciphertext = combined[nonceSize:]
	return nonce, ciphertext, nil
}

