// Package vault provides AES-256-GCM encrypted credential storage
// with a master key sealed by TPM 2.0 hardware.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/oslyak/skrynia/tpmkey"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrBadMagic   = errors.New("invalid export format: bad magic")
	ErrBadPayload = errors.New("invalid export format: cannot decode payload")
	exportMagic   = []byte{0x53, 0x4B, 0x52, 0x31} // "SKR1"
)

// Vault manages encrypted credential storage in a JSON file.
type Vault struct {
	mu       sync.Mutex
	basePath string // path without extension
	key      []byte // 32-byte AES key (unsealed from TPM)
	data     map[string]map[string]string
}

// DefaultPath returns the platform-specific vault base path (without extension).
// Files: <basePath>.key (sealed blob), <basePath>.dat (encrypted JSON).
func DefaultPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("vault: APPDATA not set")
		}
		return filepath.Join(appData, "skrynia", "vault"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("vault: cannot determine home dir: %w", err)
		}
		return filepath.Join(home, ".local", "share", "skrynia", "vault"), nil
	}
}

// Open opens or creates the vault at basePath.
// Files used: basePath.key and basePath.dat.
func Open(basePath string) (*Vault, error) {
	if !tpmkey.Available() {
		return nil, fmt.Errorf("vault: TPM 2.0 not available (on Linux, ensure user is in 'tss' group and /dev/tpmrm0 exists)")
	}

	// Strip any extension the caller may have passed (e.g. ".db" from old code)
	ext := filepath.Ext(basePath)
	if ext != "" {
		basePath = strings.TrimSuffix(basePath, ext)
	}

	dir := filepath.Dir(basePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("vault: mkdir error: %w", err)
	}

	// Load or create the master key via TPM
	key, err := loadOrCreateKey(basePath + ".key")
	if err != nil {
		return nil, err
	}

	// Load existing data or start empty
	data, err := loadData(basePath+".dat", key)
	if err != nil {
		zeroKey(key)
		return nil, err
	}

	return &Vault{
		basePath: basePath,
		key:      key,
		data:     data,
	}, nil
}

// loadOrCreateKey reads the sealed blob from keyPath, or creates a new one.
func loadOrCreateKey(keyPath string) ([]byte, error) {
	sealedBlob, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		// First run: generate, seal, and retain key in one TPM operation
		blob, key, err := tpmkey.SealNewKeyRetain()
		if err != nil {
			return nil, fmt.Errorf("vault: TPM seal failed: %w", err)
		}
		if err := os.WriteFile(keyPath, blob, 0600); err != nil {
			zeroKey(key)
			return nil, fmt.Errorf("vault: write sealed key failed: %w", err)
		}
		return key, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vault: read sealed key failed: %w", err)
	}

	key, err := tpmkey.Unseal(sealedBlob)
	if err != nil {
		return nil, fmt.Errorf("vault: TPM unseal failed: %w", err)
	}
	return key, nil
}

// loadData reads and decrypts the JSON data file, or returns an empty map.
func loadData(datPath string, key []byte) (map[string]map[string]string, error) {
	ciphertext, err := os.ReadFile(datPath)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("vault: read data failed: %w", err)
	}

	plaintext, err := decryptAESGCM(key, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt data failed: %w", err)
	}

	var data map[string]map[string]string
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("vault: unmarshal data failed: %w", err)
	}
	return data, nil
}

// save encrypts and writes the data to disk.
func (v *Vault) save() error {
	plaintext, err := json.Marshal(v.data)
	if err != nil {
		return fmt.Errorf("vault: marshal data failed: %w", err)
	}

	ciphertext, err := encryptAESGCM(v.key, plaintext)
	if err != nil {
		return fmt.Errorf("vault: encrypt data failed: %w", err)
	}

	datPath := v.basePath + ".dat"
	tmpPath := datPath + ".tmp"

	if err := os.WriteFile(tmpPath, ciphertext, 0600); err != nil {
		return fmt.Errorf("vault: write temp file failed: %w", err)
	}
	if err := os.Rename(tmpPath, datPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("vault: rename failed: %w", err)
	}
	return nil
}

// Close encrypts and writes data to disk, then zeros the master key.
func (v *Vault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	err := v.save()
	zeroKey(v.key)
	return err
}

const (
	maxKeyLen   = 256
	maxValueLen = 64 * 1024 // 64 KB
)

// Set stores a credential value and saves to disk.
func (v *Vault) Set(service, key, value string) error {
	if len(service) > maxKeyLen {
		return fmt.Errorf("service name too long (%d > %d)", len(service), maxKeyLen)
	}
	if len(key) > maxKeyLen {
		return fmt.Errorf("key name too long (%d > %d)", len(key), maxKeyLen)
	}
	if len(value) > maxValueLen {
		return fmt.Errorf("value too large (%d > %d bytes)", len(value), maxValueLen)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	isNew := v.data[service] == nil
	if isNew {
		v.data[service] = make(map[string]string)
	}
	oldValue, hadKey := v.data[service][key]
	v.data[service][key] = value

	if err := v.save(); err != nil {
		// Rollback memory state
		if hadKey {
			v.data[service][key] = oldValue
		} else {
			delete(v.data[service], key)
			if isNew {
				delete(v.data, service)
			}
		}
		return err
	}
	return nil
}

// Get retrieves a credential value.
func (v *Vault) Get(service, key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	svc, ok := v.data[service]
	if !ok {
		return "", ErrNotFound
	}
	val, ok := svc[key]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

// List returns all key names within a service (sorted), or all service names if service is empty.
func (v *Vault) List(service string) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if service == "" {
		result := make([]string, 0, len(v.data))
		for s := range v.data {
			result = append(result, s)
		}
		sort.Strings(result)
		return result, nil
	}

	svc, ok := v.data[service]
	if !ok {
		return nil, nil
	}
	result := make([]string, 0, len(svc))
	for k := range svc {
		result = append(result, k)
	}
	sort.Strings(result)
	return result, nil
}

// Delete removes a single key (if key is non-empty) or all keys of a service.
func (v *Vault) Delete(service, key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if key == "" {
		oldSvc, ok := v.data[service]
		if !ok {
			return ErrNotFound
		}
		delete(v.data, service)
		if err := v.save(); err != nil {
			v.data[service] = oldSvc // rollback
			return err
		}
	} else {
		svc, ok := v.data[service]
		if !ok {
			return ErrNotFound
		}
		oldValue, ok := svc[key]
		if !ok {
			return ErrNotFound
		}
		delete(svc, key)
		removedSvc := len(svc) == 0
		if removedSvc {
			delete(v.data, service)
		}
		if err := v.save(); err != nil {
			// rollback
			if removedSvc {
				v.data[service] = svc
			}
			svc[key] = oldValue
			return err
		}
	}
	return nil
}

// Env returns all key-value pairs for a service with normalized key names
// (uppercased, hyphens replaced with underscores).
func (v *Vault) Env(service string) (map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	svc, ok := v.data[service]
	if !ok || len(svc) == 0 {
		return nil, ErrNotFound
	}

	result := make(map[string]string, len(svc))
	for k, val := range svc {
		envKey := strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
		result[envKey] = val
	}
	return result, nil
}

type exportRecord struct {
	Service string `json:"service"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// Export returns all credentials as an encrypted binary blob.
func (v *Vault) Export() ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	var records []exportRecord
	// Sort services for deterministic output
	services := make([]string, 0, len(v.data))
	for s := range v.data {
		services = append(services, s)
	}
	sort.Strings(services)

	for _, s := range services {
		keys := make([]string, 0, len(v.data[s]))
		for k := range v.data[s] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			records = append(records, exportRecord{
				Service: s,
				Key:     k,
				Value:   v.data[s][k],
			})
		}
	}

	payload, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("vault: export marshal error: %w", err)
	}

	encrypted, err := encryptAESGCM(v.key, payload)
	if err != nil {
		return nil, fmt.Errorf("vault: export encrypt error: %w", err)
	}

	blob := make([]byte, 0, len(exportMagic)+len(encrypted))
	blob = append(blob, exportMagic...)
	blob = append(blob, encrypted...)
	return blob, nil
}

// Import decrypts a blob and merges all records into the vault.
func (v *Vault) Import(blob []byte) error {
	if !bytes.HasPrefix(blob, exportMagic) {
		return ErrBadMagic
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	encrypted := blob[len(exportMagic):]
	payload, err := decryptAESGCM(v.key, encrypted)
	if err != nil {
		return ErrBadPayload
	}

	var records []exportRecord
	if err := json.Unmarshal(payload, &records); err != nil {
		return ErrBadPayload
	}

	for _, r := range records {
		if v.data[r.Service] == nil {
			v.data[r.Service] = make(map[string]string)
		}
		v.data[r.Service][r.Key] = r.Value
	}

	return v.save()
}

// --- AES-256-GCM helpers ---

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

func zeroKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}
