package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func openTestVault(t *testing.T) *Vault {
	t.Helper()
	basePath := filepath.Join(t.TempDir(), "test")
	v, err := Open(basePath)
	require.NoError(t, err, "Open failed — is TPM 2.0 accessible?")
	t.Cleanup(func() { v.Close() })
	return v
}

func TestSetAndGet(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	err := v.Set("redmine", "password", "secret123")
	require.NoError(err)

	val, err := v.Get("redmine", "password")
	require.NoError(err)
	require.Equal("secret123", val)
}

func TestGetNotFound(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	_, err := v.Get("nonexistent", "key")
	require.ErrorIs(err, ErrNotFound)
}

func TestUpsert(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	err := v.Set("svc", "key", "value1")
	require.NoError(err)

	err = v.Set("svc", "key", "value2")
	require.NoError(err)

	val, err := v.Get("svc", "key")
	require.NoError(err)
	require.Equal("value2", val)
}

func TestList(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	_ = v.Set("alpha", "k1", "v1")
	_ = v.Set("alpha", "k2", "v2")
	_ = v.Set("beta", "k3", "v3")

	// List all services
	services, err := v.List("")
	require.NoError(err)
	require.Equal([]string{"alpha", "beta"}, services)

	// List keys within a service
	keys, err := v.List("alpha")
	require.NoError(err)
	require.Equal([]string{"k1", "k2"}, keys)
}

func TestDelete(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	_ = v.Set("svc", "k1", "v1")
	_ = v.Set("svc", "k2", "v2")

	// Delete single key
	err := v.Delete("svc", "k1")
	require.NoError(err)

	_, err = v.Get("svc", "k1")
	require.ErrorIs(err, ErrNotFound)

	// k2 still exists
	val, err := v.Get("svc", "k2")
	require.NoError(err)
	require.Equal("v2", val)

	// Delete entire service
	_ = v.Set("svc", "k3", "v3")
	err = v.Delete("svc", "")
	require.NoError(err)

	keys, err := v.List("svc")
	require.NoError(err)
	require.Empty(keys)
}

func TestDeleteNotFound(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	err := v.Delete("nonexistent", "key")
	require.ErrorIs(err, ErrNotFound)
}

func TestEnv(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	_ = v.Set("redmine", "api-key", "abc123")
	_ = v.Set("redmine", "password", "secret")

	env, err := v.Env("redmine")
	require.NoError(err)
	require.Equal("abc123", env["API_KEY"])
	require.Equal("secret", env["PASSWORD"])
}

func TestEnvNotFound(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	_, err := v.Env("nonexistent")
	require.ErrorIs(err, ErrNotFound)
}

func TestExportImport(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	_ = v.Set("svc1", "k1", "val1")
	_ = v.Set("svc2", "k2", "val2")

	blob, err := v.Export()
	require.NoError(err)
	require.True(len(blob) > 4)

	// Delete originals and re-import
	_ = v.Delete("svc1", "")
	_ = v.Delete("svc2", "")

	err = v.Import(blob)
	require.NoError(err)

	v1, err := v.Get("svc1", "k1")
	require.NoError(err)
	require.Equal("val1", v1)

	v2, err := v.Get("svc2", "k2")
	require.NoError(err)
	require.Equal("val2", v2)
}

func TestImportBadMagic(t *testing.T) {
	require := require.New(t)
	v := openTestVault(t)

	err := v.Import([]byte("BADmagicdata"))
	require.ErrorIs(err, ErrBadMagic)
}

func TestPersistence(t *testing.T) {
	require := require.New(t)
	basePath := filepath.Join(t.TempDir(), "test")

	// Open, write, close
	v1, err := Open(basePath)
	require.NoError(err)
	err = v1.Set("svc", "key", "persistent-value")
	require.NoError(err)
	err = v1.Close()
	require.NoError(err)

	// Re-open and read
	v2, err := Open(basePath)
	require.NoError(err)
	defer v2.Close()

	val, err := v2.Get("svc", "key")
	require.NoError(err)
	require.Equal("persistent-value", val)
}

func TestVaultFilesExist(t *testing.T) {
	require := require.New(t)
	basePath := filepath.Join(t.TempDir(), "test")

	v, err := Open(basePath)
	require.NoError(err)

	err = v.Set("svc", "key", "val")
	require.NoError(err)
	err = v.Close()
	require.NoError(err)

	// Both .key and .dat files should exist
	_, err = os.Stat(basePath + ".key")
	require.NoError(err, ".key file should exist")
	_, err = os.Stat(basePath + ".dat")
	require.NoError(err, ".dat file should exist")
}
