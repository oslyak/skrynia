package tpmkey

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAvailable(t *testing.T) {
	require := require.New(t)
	require.True(Available(), "TPM 2.0 must be available (user must be in tss group)")
}

func TestSealAndUnseal(t *testing.T) {
	require := require.New(t)

	blob, err := SealNewKey()
	require.NoError(err, "SealNewKey failed — is TPM accessible?")
	require.NotEmpty(blob)

	key, err := Unseal(blob)
	require.NoError(err, "Unseal failed")
	require.Len(key, 32, "master key must be 32 bytes")
}

func TestSealProducesDifferentKeys(t *testing.T) {
	require := require.New(t)

	blob1, err := SealNewKey()
	require.NoError(err)
	key1, err := Unseal(blob1)
	require.NoError(err)

	blob2, err := SealNewKey()
	require.NoError(err)
	key2, err := Unseal(blob2)
	require.NoError(err)

	require.NotEqual(key1, key2, "two SealNewKey calls should produce different keys")
}

func TestUnsealBadBlob(t *testing.T) {
	require := require.New(t)

	_, err := Unseal([]byte("bad"))
	require.Error(err, "Unseal with bad blob should fail")
}
