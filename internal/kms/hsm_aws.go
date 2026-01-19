//go:build aws
// +build aws

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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// AWSKMSProvider implements HSMProvider using AWS KMS.
type AWSKMSProvider struct {
	client   *kms.Client
	keyID    string
	dataKey  []byte // Cached data key for encryption
	aead     cipher.AEAD
	ctx      context.Context
}

// NewAWSKMSProvider creates a new AWS KMS provider.
//
// Parameters:
//   - keyID: AWS KMS Key ID or ARN (e.g., "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012")
//   - region: AWS region (e.g., "us-east-1")
func NewAWSKMSProvider(keyID, region string) (*AWSKMSProvider, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := kms.NewFromConfig(cfg)

	// Generate a data key from KMS
	// AWS KMS doesn't allow direct encryption of large data, so we use envelope encryption
	dataKeyResp, err := client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:   aws.String(keyID),
		KeySpec: types.DataKeySpec AES_256,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate data key from AWS KMS: %w", err)
	}

	dataKey := dataKeyResp.Plaintext
	if len(dataKey) != 32 {
		return nil, errors.New("AWS KMS returned invalid key size")
	}

	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AWSKMSProvider{
		client:  client,
		keyID:   keyID,
		dataKey: dataKey,
		aead:    aead,
		ctx:     ctx,
	}, nil
}

func (a *AWSKMSProvider) GetKey(keyID string) ([]byte, error) {
	// For AWS KMS, we use envelope encryption
	// Return the data key (encrypted by KMS master key)
	return a.dataKey, nil
}

func (a *AWSKMSProvider) Encrypt(keyID string, plaintext []byte) (ciphertext, nonce []byte, err error) {
	// Use software encryption with KMS-generated data key
	nonce = make([]byte, a.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = a.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func (a *AWSKMSProvider) Decrypt(keyID string, ciphertext, nonce []byte) ([]byte, error) {
	// Use software decryption with KMS-generated data key
	plaintext, err := a.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func (a *AWSKMSProvider) Close() error {
	// Clear data key from memory
	if a.dataKey != nil {
		for i := range a.dataKey {
			a.dataKey[i] = 0
		}
		a.dataKey = nil
	}
	return nil
}

