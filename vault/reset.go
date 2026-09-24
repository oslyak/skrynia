package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oslyak/skrynia/tpmkey"
)

func checkReset(basePath string) error {
	if _, err := os.Stat(basePath + ".reset"); err == nil {
		return fmt.Errorf("vault: interrupted reset; preserve all files and recover using the archive path in %s.reset", basePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Reset archives the existing encrypted files and creates an empty vault on the
// current TPM. The old key is never unsealed. The returned archive must be kept,
// including when a filesystem error interrupts activation of the new vault.
func Reset(basePath string) (string, error) {
	basePath = strings.TrimSuffix(basePath, filepath.Ext(basePath))
	if err := os.MkdirAll(filepath.Dir(basePath), 0700); err != nil {
		return "", err
	}
	lock, err := lockVault(basePath)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err := checkReset(basePath); err != nil {
		return "", err
	}
	for _, suffix := range []string{".key", ".dat"} {
		info, err := os.Lstat(basePath + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("vault: refusing to reset non-regular file %s%s", basePath, suffix)
		}
	}
	blob, key, err := tpmkey.SealNewKeyRetain()
	if err != nil {
		return "", err
	}
	defer zeroKey(key)
	verifiedKey, err := tpmkey.Unseal(blob)
	if err != nil {
		return "", err
	}
	verified := bytes.Equal(key, verifiedKey)
	zeroKey(verifiedKey)
	if !verified {
		return "", fmt.Errorf("vault: new TPM key failed verification")
	}
	data, err := encryptAESGCM(key, []byte("{}"))
	if err != nil {
		return "", err
	}
	archive, err := os.MkdirTemp(filepath.Dir(basePath), filepath.Base(basePath)+"-archive-")
	if err != nil {
		return "", err
	}
	for _, suffix := range []string{".key", ".dat"} {
		old, err := os.ReadFile(basePath + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return archive, err
		}
		if err := writeResetFile(filepath.Join(archive, filepath.Base(basePath)+suffix), old); err != nil {
			return archive, err
		}
	}
	newKeyPath := filepath.Join(archive, "new.key")
	newDataPath := filepath.Join(archive, "new.dat")
	if err := writeResetFile(newKeyPath, blob); err != nil {
		return archive, err
	}
	if err := writeResetFile(newDataPath, data); err != nil {
		return archive, err
	}
	// A marker prevents opening a mixed key/data pair if the process is interrupted.
	if err := writeResetFile(basePath+".reset", []byte(archive+"\n")); err != nil {
		return archive, err
	}
	if err := os.Rename(newKeyPath, basePath+".key"); err != nil {
		return archive, err
	}
	if err := os.Rename(newDataPath, basePath+".dat"); err != nil {
		return archive, err
	}
	return archive, os.Remove(basePath + ".reset")
}

func writeResetFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}
