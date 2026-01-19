//go:build pkcs11
// +build pkcs11

package kms

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/miekg/pkcs11"
)

// PKCS11Provider implements HSMProvider using PKCS#11 interface.
type PKCS11Provider struct {
	ctx      *pkcs11.Ctx
	session  pkcs11.SessionHandle
	slotID   uint
	pin      string
	keyLabel string
	mu       sync.Mutex // PKCS#11 sessions are not goroutine-safe
}

// NewPKCS11Provider creates a new PKCS#11 HSM provider.
func NewPKCS11Provider(libPath string, slotID uint, pin, keyLabel string) (*PKCS11Provider, error) {
	ctx := pkcs11.New(libPath)
	if ctx == nil {
		return nil, errors.New("failed to load PKCS#11 library")
	}

	err := ctx.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PKCS#11: %w", err)
	}

	// --- NEW LOGIC: FIND SLOT BY LABEL ---
	// We ignore the 'slotID' input if we can find the slot by label "KMS Token".
	// Or we just list all slots and try to find the one with the Token inside.

	slots, err := ctx.GetSlotList(true) // Get slots with tokens present
	if err != nil {
		ctx.Finalize()
		return nil, fmt.Errorf("failed to get slot list: %w", err)
	}

	var targetSlot uint
	var found bool

	fmt.Printf("DEBUG: Found %d slots with tokens:\n", len(slots))

	// Iterate over all available slots to find the right one
	for _, slot := range slots {
		tokenInfo, err := ctx.GetTokenInfo(slot)
		if err != nil {
			continue
		}
		fmt.Printf("  - Checking Slot ID %d (Label: %s)\n", slot, tokenInfo.Label)

		// Check if this token matches our expectations.
		// Usually, SoftHSM labels are padded with spaces, so checking connection is key.
		// If you know the token label is "KMS Token", you can check strict equality.
		// For now, let's try to use the slotID passed in env IF it matches, otherwise use the first valid one.

		if uint(slot) == slotID {
			targetSlot = slot
			found = true
			fmt.Println("    -> MATCHED user-provided Slot ID")
			break
		}
	}

	// If user provided ID didn't match any list, just pick the first one (common fix)
	// OR if you want to be specific, check the token Label "KMS Token"
	if !found {
		if len(slots) > 0 {
			fmt.Printf("DEBUG: Slot ID %d not found in list. Using first available slot: %d\n", slotID, slots[0])
			targetSlot = slots[0]
			found = true
		} else {
			ctx.Finalize()
			return nil, errors.New("no slots with tokens found")
		}
	}

	// -------------------------------------

	// Open Session using the CONFIRMED slot ID
	session, err := ctx.OpenSession(targetSlot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		ctx.Finalize()
		return nil, fmt.Errorf("failed to open PKCS#11 session on slot %d: %w", targetSlot, err)
	}

	// Login
	err = ctx.Login(session, pkcs11.CKU_USER, pin)
	if err != nil {
		ctx.CloseSession(session)
		ctx.Finalize()
		return nil, fmt.Errorf("failed to login to PKCS#11: %w", err)
	}

	return &PKCS11Provider{
		ctx:      ctx,
		session:  session,
		slotID:   targetSlot, // Use the real one
		pin:      pin,
		keyLabel: keyLabel,
	}, nil
}

// findKeyHandle looks up the object handle for the key inside the HSM.
// It does NOT extract the key data.
func (p *PKCS11Provider) findKeyHandle() (pkcs11.ObjectHandle, error) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, p.keyLabel),
	}

	if err := p.ctx.FindObjectsInit(p.session, template); err != nil {
		return 0, err
	}
	defer p.ctx.FindObjectsFinal(p.session)

	objs, _, err := p.ctx.FindObjects(p.session, 1)
	if err != nil {
		return 0, err
	}
	if len(objs) == 0 {
		return 0, errors.New("key not found in HSM")
	}

	return objs[0], nil
}

// GetKey is DISABLED because HSM keys are not extractable.
// You must use Encrypt/Decrypt methods instead.
func (p *PKCS11Provider) GetKey(keyID string) ([]byte, error) {
	return nil, errors.New("security violation: cannot extract raw CKA_SENSITIVE key from HSM")
}

// Encrypt performs AES-GCM encryption inside the HSM.
func (p *PKCS11Provider) Encrypt(keyID string, plaintext []byte) ([]byte, []byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	keyHandle, err := p.findKeyHandle()
	if err != nil {
		return nil, nil, err
	}

	// 1. Generate a 12-byte Nonce (IV) locally
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %v", err)
	}

	// 2. Configure AES-GCM Mechanism (128-bit tag)
	gcmParams := pkcs11.NewGCMParams(nonce, nil, 128)
	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_AES_GCM, gcmParams)}

	// 3. Initialize Encryption
	if err := p.ctx.EncryptInit(p.session, mech, keyHandle); err != nil {
		return nil, nil, fmt.Errorf("encrypt init failed: %w", err)
	}

	// 4. Perform Encryption
	ciphertext, err := p.ctx.Encrypt(p.session, plaintext)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt execution failed: %w", err)
	}

	return ciphertext, nonce, nil
}

// Decrypt performs AES-GCM decryption inside the HSM.
func (p *PKCS11Provider) Decrypt(keyID string, ciphertext, nonce []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	keyHandle, err := p.findKeyHandle()
	if err != nil {
		return nil, err
	}

	// 1. Configure AES-GCM with the nonce received during encryption
	gcmParams := pkcs11.NewGCMParams(nonce, nil, 128)
	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_AES_GCM, gcmParams)}

	// 2. Initialize Decryption
	if err := p.ctx.DecryptInit(p.session, mech, keyHandle); err != nil {
		return nil, fmt.Errorf("decrypt init failed: %w", err)
	}

	// 3. Perform Decryption
	plaintext, err := p.ctx.Decrypt(p.session, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt execution failed: %w", err)
	}

	return plaintext, nil
}

// Close cleans up the session.
func (p *PKCS11Provider) Close() error {
	if p.ctx == nil {
		return nil
	}

	if p.session != 0 {
		p.ctx.Logout(p.session)
		p.ctx.CloseSession(p.session)
	}

	p.ctx.Finalize()
	return nil
}
