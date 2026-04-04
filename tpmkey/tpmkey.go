// Package tpmkey provides TPM 2.0 based key sealing and unsealing.
// The master key is sealed under the TPM's Storage Root Key (SRK) and
// can only be unsealed on the same physical TPM.
package tpmkey

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

const keyLen = 32

// sealedBlob format: [2 bytes privLen][private][public]
// This is a simple format to store both TPM2BPrivate and TPM2BPublic.

// SealNewKey generates a 32-byte random key and seals it with the TPM.
// Returns the sealed blob for storage.
// SealNewKey generates a 32-byte random key, seals it with the TPM, and zeros
// the plaintext key. Returns the sealed blob only. Use SealNewKeyRetain if you
// need the plaintext key immediately after sealing (avoids a redundant Unseal).
func SealNewKey() ([]byte, error) {
	blob, _, err := sealNewKey(false)
	return blob, err
}

// SealNewKeyRetain generates and seals a key, returning both the sealed blob
// and the plaintext key. Caller is responsible for zeroing the returned key.
func SealNewKeyRetain() (blob []byte, key []byte, err error) {
	return sealNewKey(true)
}

func sealNewKey(retain bool) ([]byte, []byte, error) {
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, fmt.Errorf("tpmkey: random generation failed: %w", err)
	}

	tpm, err := openTPM()
	if err != nil {
		for i := range key {
			key[i] = 0
		}
		return nil, nil, fmt.Errorf("tpmkey: cannot open TPM: %w", err)
	}
	defer tpm.Close()

	blob, err := sealKey(tpm, key)
	if err != nil {
		for i := range key {
			key[i] = 0
		}
		return nil, nil, err
	}

	if retain {
		return blob, key, nil
	}
	for i := range key {
		key[i] = 0
	}
	return blob, nil, nil
}

// Unseal decrypts a sealed blob using the TPM.
// Returns the 32-byte master key.
func Unseal(sealedBlob []byte) ([]byte, error) {
	tpm, err := openTPM()
	if err != nil {
		return nil, fmt.Errorf("tpmkey: cannot open TPM: %w", err)
	}
	defer tpm.Close()

	return unsealKey(tpm, sealedBlob)
}

// Available checks if the TPM is accessible on this machine.
func Available() bool {
	tpm, err := openTPM()
	if err != nil {
		return false
	}
	tpm.Close()
	return true
}

// createSRK creates a primary key (SRK) under the owner hierarchy.
// The SRK is transient and must be flushed after use.
func createSRK(tpm transport.TPM) (*tpm2.CreatePrimaryResponse, error) {
	cmd := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.TPMRHOwner,
		InPublic:      tpm2.New2B(tpm2.ECCSRKTemplate),
	}
	rsp, err := cmd.Execute(tpm)
	if err != nil {
		return nil, fmt.Errorf("tpmkey: CreatePrimary failed: %w", err)
	}
	return rsp, nil
}

// sealKey seals the given key data under the SRK.
func sealKey(tpm transport.TPM, key []byte) ([]byte, error) {
	srk, err := createSRK(tpm)
	if err != nil {
		return nil, err
	}
	defer func() {
		flush := tpm2.FlushContext{FlushHandle: srk.ObjectHandle}
		flush.Execute(tpm)
	}()

	// Create a sealed object under the SRK
	createCmd := tpm2.Create{
		ParentHandle: tpm2.AuthHandle{
			Handle: srk.ObjectHandle,
			Name:   srk.Name,
			Auth:   tpm2.PasswordAuth(nil),
		},
		InSensitive: tpm2.TPM2BSensitiveCreate{
			Sensitive: &tpm2.TPMSSensitiveCreate{
				Data: tpm2.NewTPMUSensitiveCreate(&tpm2.TPM2BSensitiveData{
					Buffer: key,
				}),
			},
		},
		InPublic: tpm2.New2B(tpm2.TPMTPublic{
			Type:    tpm2.TPMAlgKeyedHash,
			NameAlg: tpm2.TPMAlgSHA256,
			ObjectAttributes: tpm2.TPMAObject{
				FixedTPM:     true,
				FixedParent:  true,
				UserWithAuth: true,
				NoDA:         true,
			},
		}),
	}

	createRsp, err := createCmd.Execute(tpm)
	if err != nil {
		return nil, fmt.Errorf("tpmkey: Create (seal) failed: %w", err)
	}

	return encodeBlob(createRsp.OutPrivate, createRsp.OutPublic), nil
}

// unsealKey unseals the key from a blob.
func unsealKey(tpm transport.TPM, blob []byte) ([]byte, error) {
	priv, pub, err := decodeBlob(blob)
	if err != nil {
		return nil, err
	}

	srk, err := createSRK(tpm)
	if err != nil {
		return nil, err
	}
	defer func() {
		flush := tpm2.FlushContext{FlushHandle: srk.ObjectHandle}
		flush.Execute(tpm)
	}()

	// Load the sealed object
	loadCmd := tpm2.Load{
		ParentHandle: tpm2.AuthHandle{
			Handle: srk.ObjectHandle,
			Name:   srk.Name,
			Auth:   tpm2.PasswordAuth(nil),
		},
		InPrivate: priv,
		InPublic:  pub,
	}
	loadRsp, err := loadCmd.Execute(tpm)
	if err != nil {
		return nil, fmt.Errorf("tpmkey: Load failed: %w", err)
	}
	defer func() {
		flush := tpm2.FlushContext{FlushHandle: loadRsp.ObjectHandle}
		flush.Execute(tpm)
	}()

	// Unseal
	unsealCmd := tpm2.Unseal{
		ItemHandle: tpm2.AuthHandle{
			Handle: loadRsp.ObjectHandle,
			Name:   loadRsp.Name,
			Auth:   tpm2.PasswordAuth(nil),
		},
	}
	unsealRsp, err := unsealCmd.Execute(tpm)
	if err != nil {
		return nil, fmt.Errorf("tpmkey: Unseal failed: %w", err)
	}

	key := make([]byte, len(unsealRsp.OutData.Buffer))
	copy(key, unsealRsp.OutData.Buffer)
	return key, nil
}

// encodeBlob serializes TPM2BPrivate and TPM2BPublic into a single byte slice.
// Format: [2 bytes big-endian privLen][private bytes][public bytes]
func encodeBlob(priv tpm2.TPM2BPrivate, pub tpm2.TPM2BPublic) []byte {
	privBytes := tpm2.Marshal(priv)
	pubBytes := tpm2.Marshal(pub)

	buf := make([]byte, 2+len(privBytes)+len(pubBytes))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(privBytes)))
	copy(buf[2:], privBytes)
	copy(buf[2+len(privBytes):], pubBytes)
	return buf
}

// decodeBlob deserializes a blob back into TPM2BPrivate and TPM2BPublic.
func decodeBlob(blob []byte) (tpm2.TPM2BPrivate, tpm2.TPM2BPublic, error) {
	if len(blob) < 4 {
		return tpm2.TPM2BPrivate{}, tpm2.TPM2BPublic{}, fmt.Errorf("tpmkey: sealed blob too short")
	}

	privLen := int(binary.BigEndian.Uint16(blob[0:2]))
	if 2+privLen > len(blob) {
		return tpm2.TPM2BPrivate{}, tpm2.TPM2BPublic{}, fmt.Errorf("tpmkey: sealed blob corrupted (privLen=%d, total=%d)", privLen, len(blob))
	}

	privBytes := blob[2 : 2+privLen]
	pubBytes := blob[2+privLen:]

	priv, err := tpm2.Unmarshal[tpm2.TPM2BPrivate](privBytes)
	if err != nil {
		return tpm2.TPM2BPrivate{}, tpm2.TPM2BPublic{}, fmt.Errorf("tpmkey: cannot decode private: %w", err)
	}

	pub, err := tpm2.Unmarshal[tpm2.TPM2BPublic](pubBytes)
	if err != nil {
		return tpm2.TPM2BPrivate{}, tpm2.TPM2BPublic{}, fmt.Errorf("tpmkey: cannot decode public: %w", err)
	}

	return *priv, *pub, nil
}
