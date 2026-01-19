package kms

import (
	"crypto/cipher"

	"errors"

	"fmt"

	"sync"
)

// HSMProvider defines the interface for Hardware Security Module providers.

// This allows the KMS to use different HSM backends (PKCS#11, Cloud KMS, etc.)

type HSMProvider interface {

	// GetKey retrieves the master key from HSM.

	// The key should be 32 bytes for AES-256.

	GetKey(keyID string) ([]byte, error)

	// Encrypt performs encryption using HSM (if HSM supports it directly).

	// Returns nil if HSM doesn't support direct encryption (fallback to software).

	Encrypt(keyID string, plaintext []byte) (ciphertext, nonce []byte, err error)

	// Decrypt performs decryption using HSM (if HSM supports it directly).

	// Returns nil if HSM doesn't support direct decryption (fallback to software).

	Decrypt(keyID string, ciphertext, nonce []byte) ([]byte, error)

	// Close releases HSM resources.

	Close() error
}

// HSMManager wraps HSMProvider and provides encryption/decryption using HSM keys.

type HSMManager struct {
	mu sync.RWMutex

	provider HSMProvider

	keyID string

	aead cipher.AEAD // Cached AEAD for software encryption with HSM key

}

// NewHSMManager creates a new HSM-backed manager.
func NewHSMManager(provider HSMProvider, keyID string) (*HSMManager, error) {
	// 1. Validate inputs
	if provider == nil {

		return nil, errors.New("HSM provider cannot be nil")

	}

	// 2. Perform a Health Check

	// Instead of downloading the key (unsafe), we ask the HSM to encrypt "ping".

	// If this works, we know the HSM is connected and the key exists.

	_, _, err := provider.Encrypt(keyID, []byte("ping"))

	if err != nil {

		return nil, fmt.Errorf("HSM self-test failed (check Slot ID and Label): %w", err)

	}

	// 3. Return the manager

	// Note: We REMOVED the 'aead' field because encryption now happens

	// inside the 'provider', not in this struct.

	return &HSMManager{

		provider: provider,

		keyID: keyID,
	}, nil

}

// Encrypt encrypts plaintext using HSM.

func (m *HSMManager) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {

	// FIX 1: Use Lock(), not RLock().

	// This ensures only ONE request hits the HSM at a time.

	m.mu.Lock()

	defer m.mu.Unlock()

	// Try HSM direct encryption

	ct, n, e := m.provider.Encrypt(m.keyID, plaintext)

	if e != nil {

		return nil, nil, e // Return the error directly

	}

	// FIX 2: Do not use m.aead fallback if it wasn't initialized.

	// If provider returns success, return it.

	return ct, n, nil

}

// Decrypt decrypts ciphertext using HSM.

func (m *HSMManager) Decrypt(ciphertext, nonce []byte) ([]byte, error) {

	// FIX 1: Use Lock(), not RLock().

	m.mu.Lock()

	defer m.mu.Unlock()

	// Try HSM direct decryption

	pt, e := m.provider.Decrypt(m.keyID, ciphertext, nonce)

	if e != nil {

		return nil, e

	}

	return pt, nil

}

// Close releases HSM resources.

func (m *HSMManager) Close() error {

	if m.provider != nil {

		return m.provider.Close()

	}

	return nil

}
