package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPortableExportAcrossVaults(t *testing.T) {
	source := openTestVault(t)
	target := openTestVault(t)
	password := []byte("a long independent recovery passphrase")
	require.NoError(t, source.Set("service", "password", "secret value"))
	require.NoError(t, target.Set("existing", "key", "keep me"))
	require.NoError(t, target.Set("service", "password", "old value"))
	require.NotEqual(t, source.key, target.key)

	blob, err := source.ExportWithPassword(password)
	require.NoError(t, err)
	require.True(t, IsPortableExport(blob))
	require.False(t, bytes.Contains(blob, []byte("secret value")))
	second, err := source.ExportWithPassword(password)
	require.NoError(t, err)
	require.NotEqual(t, blob, second)

	require.NoError(t, target.ImportWithPassword(blob, password))
	value, err := target.Get("service", "password")
	require.NoError(t, err)
	require.Equal(t, "secret value", value)
	value, err = target.Get("existing", "key")
	require.NoError(t, err)
	require.Equal(t, "keep me", value)

	legacy, err := source.Export()
	require.NoError(t, err)
	require.Error(t, target.Import(legacy))
	require.NoError(t, source.Import(legacy))
}

func TestPortableImportRejectsWrongPasswordAndCorruption(t *testing.T) {
	source := openTestVault(t)
	target := openTestVault(t)
	require.NoError(t, source.Set("source", "secret", "do not import on error"))
	require.NoError(t, target.Set("existing", "key", "unchanged"))
	password := []byte("test recovery passphrase")
	blob, err := source.ExportWithPassword(password)
	require.NoError(t, err)
	before, err := os.ReadFile(target.basePath + ".dat")
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		blob []byte
		pass []byte
	}{
		{"wrong-password", blob, []byte("wrong password")},
		{"empty-password", blob, nil},
		{"truncated", blob[:10], password},
		{"wrong-magic", append([]byte("NOPE"), blob[4:]...), password},
		{"tampered-salt", append(append([]byte{}, blob[:4]...), append([]byte{blob[4] ^ 1}, blob[5:]...)...), password},
		{"tampered-tag", append(append([]byte{}, blob[:len(blob)-1]...), blob[len(blob)-1]^1), password},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, target.ImportWithPassword(tc.blob, tc.pass))
			services, err := target.List("")
			require.NoError(t, err)
			require.Equal(t, []string{"existing"}, services)
			after, err := os.ReadFile(target.basePath + ".dat")
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
	_, err = source.ExportWithPassword(nil)
	require.Error(t, err)
}

func TestResetWithLostKeyArchivesBothFiles(t *testing.T) {
	base := filepath.Join(t.TempDir(), "vault")
	oldKey := []byte("unusable key from the previous TPM")
	oldData := []byte("encrypted data that must be preserved")
	require.NoError(t, os.WriteFile(base+".key", oldKey, 0600))
	require.NoError(t, os.WriteFile(base+".dat", oldData, 0600))
	_, err := Open(base)
	require.Error(t, err)

	archive, err := Reset(base)
	require.NoError(t, err)
	for suffix, expected := range map[string][]byte{".key": oldKey, ".dat": oldData} {
		path := filepath.Join(archive, "vault"+suffix)
		actual, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		}
	}
	v, err := Open(base)
	require.NoError(t, err)
	services, err := v.List("")
	require.NoError(t, err)
	require.Empty(t, services)
	require.NoError(t, v.Set("sudo", "password", "new test password"))
	require.NoError(t, v.Close())
	v, err = Open(base)
	require.NoError(t, err)
	defer v.Close()
	value, err := v.Get("sudo", "password")
	require.NoError(t, err)
	require.Equal(t, "new test password", value)
}

func TestResetAndOpenRespectActiveVaultLock(t *testing.T) {
	v := openTestVault(t)
	require.NoError(t, v.Set("service", "key", "value"))
	_, err := Open(v.basePath)
	require.ErrorIs(t, err, ErrLocked)
	_, err = Reset(v.basePath)
	require.ErrorIs(t, err, ErrLocked)
	require.NoError(t, v.Close())
	require.NoError(t, v.Close())
	_, err = Reset(v.basePath)
	require.NoError(t, err)
}

func TestInterruptedResetBlocksOpenAndReset(t *testing.T) {
	base := filepath.Join(t.TempDir(), "vault")
	require.NoError(t, os.WriteFile(base+".reset", []byte("archive-location"), 0600))
	_, err := Open(base)
	require.ErrorContains(t, err, "interrupted reset")
	_, err = Reset(base)
	require.ErrorContains(t, err, "interrupted reset")
	_, err = os.Stat(base + ".key")
	require.True(t, os.IsNotExist(err))
}

func TestMissingKeyDoesNotOverwriteExistingData(t *testing.T) {
	base := filepath.Join(t.TempDir(), "vault")
	data := []byte("old encrypted data")
	require.NoError(t, os.WriteFile(base+".dat", data, 0600))
	_, err := Open(base)
	require.ErrorContains(t, err, "data exists without its key")
	_, err = os.Stat(base + ".key")
	require.True(t, os.IsNotExist(err))
	after, err := os.ReadFile(base + ".dat")
	require.NoError(t, err)
	require.Equal(t, data, after)
}

func TestResetRejectsNonRegularFiles(t *testing.T) {
	base := filepath.Join(t.TempDir(), "vault")
	require.NoError(t, os.Mkdir(base+".dat", 0700))
	_, err := Reset(base)
	require.ErrorContains(t, err, "non-regular file")
	_, err = os.Stat(base + ".key")
	require.True(t, os.IsNotExist(err))
}
