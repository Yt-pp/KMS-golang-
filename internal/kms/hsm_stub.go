//go:build !pkcs11 && !aws && !azure
// +build !pkcs11,!aws,!azure

package kms

import "errors"

// Stub implementations for PKCS#11 when no HSM build tags are set.
// AWS/Azure stubs are provided in their own files with !aws / !azure tags.

func NewPKCS11Provider(libPath string, slotID uint, pin, keyLabel string) (HSMProvider, error) {
	return nil, errors.New("PKCS#11 support not compiled (use build tag: pkcs11)")
}
