package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResetConfirmation(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{"RESET\n", true}, {"RESET\r\n", true},
		{"\n", false}, {"yes\n", false}, {"reset\n", false}, {"RESET", false}, {"", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			require.Equal(t, tc.want, confirmReset(strings.NewReader(tc.input), io.Discard))
		})
	}
}

func TestRecoveryArgumentValidation(t *testing.T) {
	for _, args := range [][]string{
		{"list", "one", "two"}, {"reset", "unexpected"}, {"reset", "--unknown"},
		{"export", "--legacy", "--password-file", "password"},
		{"export", "one", "two"}, {"import", "--unknown"},
	} {
		require.Equal(t, 2, run(args), "%v", args)
	}
}

func TestBackupFileCannotOverwriteAndPasswordWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.enc")
	require.NoError(t, writeBackup(path, []byte("original")))
	require.Error(t, writeBackup(path, []byte("replacement")))
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "original", string(actual))

	passwordPath := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(passwordPath, []byte("  spaces are part of the password  \r\n"), 0600))
	password, err := readBackupPassword(passwordPath, false)
	require.NoError(t, err)
	require.Equal(t, "  spaces are part of the password  ", string(password))
	clear(password)
	for _, data := range [][]byte{nil, []byte("\n"), bytes.Repeat([]byte{'x'}, 4097)} {
		require.NoError(t, os.WriteFile(passwordPath, data, 0600))
		_, err = readBackupPassword(passwordPath, false)
		require.Error(t, err)
	}
}
