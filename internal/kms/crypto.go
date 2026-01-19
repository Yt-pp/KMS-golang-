package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
)

// FileManager handles loading the master key from file and performing encryption / decryption.
// This is the default implementation for file-based keys.
type FileManager struct {
	mu        sync.RWMutex
	aead      cipher.AEAD
	masterKey []byte
}

// NewManagerFromFile loads the master key from a local file.
//
// The file should contain a hex-encoded 32-byte (256-bit) key, e.g.:
//   7b6f3c... (64 hex chars)
func NewManagerFromFile(path string) (*FileManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	trimmed := string(bytesTrimSpace(data))
	key, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &FileManager{
		aead:      aead,
		masterKey: key,
	}, nil
}

// Encrypt encrypts the given plaintext using AES-GCM with a random nonce.
// It returns the ciphertext and nonce.
func (m *FileManager) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.aead == nil {
		return nil, nil, errors.New("kms manager not initialized")
	}

	nonce = make([]byte, m.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = m.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts the given ciphertext using AES-GCM and the provided nonce.
func (m *FileManager) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.aead == nil {
		return nil, errors.New("kms manager not initialized")
	}

	plaintext, err := m.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// Close releases resources (no-op for file-based manager).
func (m *FileManager) Close() error {
	// Clear master key from memory
	if m.masterKey != nil {
		for i := range m.masterKey {
			m.masterKey[i] = 0
		}
		m.masterKey = nil
	}
	return nil
}

// bytesTrimSpace is a minimal reimplementation of bytes.TrimSpace to avoid
// importing the bytes package just for this.
func bytesTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) {
		if b[start] != ' ' && b[start] != '\n' && b[start] != '\r' && b[start] != '\t' {
			break
		}
		start++
	}
	end := len(b)
	for end > start {
		if b[end-1] != ' ' && b[end-1] != '\n' && b[end-1] != '\r' && b[end-1] != '\t' {
			break
		}
		end--
	}
	return b[start:end]
}


