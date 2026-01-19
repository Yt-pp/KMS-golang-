//go:build !aws
// +build !aws

package kms

import "errors"

// Stub when aws build tag is not set.
func NewAWSKMSProvider(keyID, region string) (HSMProvider, error) {
	return nil, errors.New("AWS KMS support not compiled (use build tag: aws)")
}


