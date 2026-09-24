package vault

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// SKR2 uses PBKDF2-HMAC-SHA256 (600,000 iterations, 16-byte salt), then AES-256-GCM.
// The layout is magic | salt | nonce | ciphertext | authentication tag.
const portableMagic = "SKR2"
const exportSaltSize = 16
const exportIterations = 600_000

func IsPortableExport(blob []byte) bool {
	return bytes.HasPrefix(blob, []byte(portableMagic))
}

// ExportWithPassword creates a backup that can be restored on a different TPM.
func (v *Vault) ExportWithPassword(password []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("vault: export password must not be empty")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	payload, err := v.exportPayload()
	if err != nil {
		return nil, err
	}
	defer zeroKey(payload)
	salt := make([]byte, exportSaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key, err := pbkdf2.Key(sha256.New, string(password), salt, exportIterations, 32)
	if err != nil {
		return nil, err
	}
	defer zeroKey(key)
	encrypted, err := encryptAESGCM(key, payload)
	if err != nil {
		return nil, err
	}
	blob := append([]byte(portableMagic), salt...)
	return append(blob, encrypted...), nil
}

// ImportWithPassword merges a portable backup without using the source TPM key.
func (v *Vault) ImportWithPassword(blob, password []byte) error {
	if !IsPortableExport(blob) {
		return ErrBadMagic
	}
	if len(password) == 0 || len(blob) < len(portableMagic)+exportSaltSize+12+16 {
		return ErrBadPayload
	}
	salt := blob[len(portableMagic) : len(portableMagic)+exportSaltSize]
	key, err := pbkdf2.Key(sha256.New, string(password), salt, exportIterations, 32)
	if err != nil {
		return err
	}
	defer zeroKey(key)
	payload, err := decryptAESGCM(key, blob[len(portableMagic)+exportSaltSize:])
	if err != nil {
		return fmt.Errorf("vault: incorrect export password or damaged backup: %w", ErrBadPayload)
	}
	defer zeroKey(payload)
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.importPayload(payload)
}
