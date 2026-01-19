package kms

import (
	"errors"
	"fmt"
	"os"
)

// Manager interface defines the encryption/decryption operations.
type Manager interface {
	Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error)
	Decrypt(ciphertext, nonce []byte) ([]byte, error)
	Close() error
}

// NewManager creates a Manager based on configuration.
// It supports:
//   - File-based keys (default)
//   - HSM providers (PKCS#11, AWS KMS, Azure Key Vault)
func NewManager() (Manager, error) {
	// Check for HSM configuration
	hsmType := os.Getenv("KMS_HSM_TYPE")
	if hsmType != "" {
		return NewHSMManagerFromEnv()
	}

	// Default: file-based key
	masterKeyPath := getenvDefault("KMS_MASTER_KEY_PATH", "master.key")
	fileMgr, err := NewManagerFromFile(masterKeyPath)
	if err != nil {
		return nil, err
	}
	return fileMgr, nil
}

// NewHSMManagerFromEnv creates an HSM manager from environment variables.
func NewHSMManagerFromEnv() (Manager, error) {
	hsmType := os.Getenv("KMS_HSM_TYPE")

	switch hsmType {
	case "pkcs11":
		return NewPKCS11ManagerFromEnv()
	case "aws":
		return NewAWSKMSManagerFromEnv()
	case "azure":
		return NewAzureKeyVaultManagerFromEnv()
	default:
		return nil, errors.New("unsupported HSM type: " + hsmType)
	}
}

// NewPKCS11ManagerFromEnv creates a PKCS#11 manager from environment variables.
// In internal/kms/manager.go

func NewPKCS11ManagerFromEnv() (Manager, error) {
	libPath := os.Getenv("KMS_PKCS11_LIB")
	slotID := getenvUint("KMS_PKCS11_SLOT", 0)
	pin := os.Getenv("KMS_PKCS11_PIN")
	keyLabel := getenvDefault("KMS_PKCS11_KEY_LABEL", "kms-master-key")

	// DEBUG: Print what we are trying to load
	fmt.Printf("DEBUG: Initializing PKCS11...\n")
	fmt.Printf("  Lib: %s\n", libPath)
	fmt.Printf("  Slot: %d\n", slotID)
	fmt.Printf("  Label: %s\n", keyLabel)

	if libPath == "" {
		return nil, errors.New("KMS_PKCS11_LIB environment variable is required")
	}

	provider, err := NewPKCS11Provider(libPath, slotID, pin, keyLabel)
	if err != nil {
		return nil, fmt.Errorf("provider initialization failed: %w", err)
	}

	// CRITICAL SAFETY CHECK
	if provider == nil {
		return nil, errors.New("provider initialization returned nil pointer without error (Check PKCS11 library path)")
	}

	keyID := getenvDefault("KMS_KEY_ID", "default")
	return NewHSMManager(provider, keyID)
}

// NewAWSKMSManagerFromEnv creates an AWS KMS manager from environment variables.
func NewAWSKMSManagerFromEnv() (Manager, error) {
	keyID := os.Getenv("KMS_AWS_KEY_ID")
	region := getenvDefault("KMS_AWS_REGION", "us-east-1")

	if keyID == "" {
		return nil, errors.New("KMS_AWS_KEY_ID environment variable is required")
	}

	provider, err := NewAWSKMSProvider(keyID, region)
	if err != nil {
		return nil, err
	}

	return NewHSMManager(provider, keyID)
}

// NewAzureKeyVaultManagerFromEnv creates an Azure Key Vault manager from environment variables.
func NewAzureKeyVaultManagerFromEnv() (Manager, error) {
	vaultURL := os.Getenv("KMS_AZURE_VAULT_URL")
	keyName := os.Getenv("KMS_AZURE_KEY_NAME")

	if vaultURL == "" {
		return nil, errors.New("KMS_AZURE_VAULT_URL environment variable is required")
	}
	if keyName == "" {
		return nil, errors.New("KMS_AZURE_KEY_NAME environment variable is required")
	}

	provider, err := NewAzureKeyVaultProvider(vaultURL, keyName)
	if err != nil {
		return nil, err
	}

	return NewHSMManager(provider, keyName)
}

// Helper functions
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvUint(key string, def uint) uint {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	// Simple parsing - in production use strconv.ParseUint
	var result uint
	for _, c := range v {
		if c >= '0' && c <= '9' {
			result = result*10 + uint(c-'0')
		} else {
			return def
		}
	}
	return result
}
