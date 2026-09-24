package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/oslyak/skrynia/vault"
	"golang.org/x/term"
)

const maxBackupSize = 64 << 20

func runTransfer(command string, args []string) int {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	passwordFile := flags.String("password-file", "", "read backup password from a file")
	legacy := false
	if command == "export" {
		flags.BoolVar(&legacy, "legacy", false, "export old TPM-bound format")
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 || (legacy && *passwordFile != "") {
		fmt.Fprintln(os.Stderr, "error: expected at most one file; --legacy cannot be combined with --password-file")
		return 2
	}
	if command == "export" && flags.NArg() == 0 && term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, "error: specify an export file or redirect stdout")
		return 2
	}
	var blob []byte
	if command == "import" {
		input := os.Stdin
		if flags.NArg() == 1 {
			f, err := os.Open(flags.Arg(0))
			if err != nil {
				return reportError(err)
			}
			defer f.Close()
			input = f
		}
		var err error
		blob, err = io.ReadAll(io.LimitReader(input, maxBackupSize+1))
		if err != nil {
			return reportError(err)
		}
		if len(blob) > maxBackupSize {
			return reportError(fmt.Errorf("backup exceeds %d bytes", maxBackupSize))
		}
		if !vault.IsPortableExport(blob) && !bytes.HasPrefix(blob, []byte("SKR1")) {
			return reportError(vault.ErrBadMagic)
		}
	}
	return withVault(func(v *vault.Vault) int {
		var password []byte
		if (command == "export" && !legacy) || vault.IsPortableExport(blob) {
			var err error
			password, err = readBackupPassword(*passwordFile, command == "export")
			if err != nil {
				return reportError(err)
			}
			defer clear(password)
		} else if *passwordFile != "" {
			return reportError(fmt.Errorf("legacy backups require their original TPM key, not a password"))
		}
		if command == "import" {
			var err error
			if vault.IsPortableExport(blob) {
				err = v.ImportWithPassword(blob, password)
			} else {
				err = v.Import(blob)
			}
			if err != nil {
				return reportError(err)
			}
			return 0
		}
		var err error
		if legacy {
			fmt.Fprintln(os.Stderr, "Warning: legacy export requires the original TPM key and cannot recover from TPM loss.")
			blob, err = v.Export()
		} else {
			blob, err = v.ExportWithPassword(password)
		}
		if err != nil {
			return reportError(err)
		}
		if flags.NArg() == 1 {
			return reportError(writeBackup(flags.Arg(0), blob))
		}
		_, err = os.Stdout.Write(blob)
		return reportError(err)
	})
}

func writeBackup(path string, blob []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(blob)
	syncErr := f.Sync()
	closeErr := f.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func openTerminal() (*os.File, error) {
	path := "/dev/tty"
	if runtime.GOOS == "windows" {
		path = "CONIN$"
	}
	return os.OpenFile(path, os.O_RDWR, 0)
}

func readBackupPassword(path string, confirm bool) ([]byte, error) {
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		password, err := io.ReadAll(io.LimitReader(f, 4097))
		if err != nil || len(password) > 4096 {
			clear(password)
			return nil, fmt.Errorf("cannot read password file (maximum 4096 bytes)")
		}
		password = bytes.TrimSuffix(password, []byte("\n"))
		password = bytes.TrimSuffix(password, []byte("\r"))
		if len(password) == 0 {
			return nil, fmt.Errorf("backup password must not be empty")
		}
		return password, nil
	}
	tty, err := openTerminal()
	if err != nil {
		return nil, fmt.Errorf("no terminal for password entry; use --password-file: %w", err)
	}
	defer tty.Close()
	fmt.Fprint(os.Stderr, "Backup password: ")
	password, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil || len(password) == 0 {
		clear(password)
		return nil, fmt.Errorf("could not read a non-empty backup password")
	}
	if confirm {
		fmt.Fprint(os.Stderr, "Repeat backup password: ")
		repeated, err := term.ReadPassword(int(tty.Fd()))
		fmt.Fprintln(os.Stderr)
		matches := bytes.Equal(password, repeated)
		clear(repeated)
		if err != nil || !matches {
			clear(password)
			return nil, fmt.Errorf("backup passwords do not match")
		}
	}
	return password, nil
}

func reportError(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}
