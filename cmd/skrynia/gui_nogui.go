//go:build nogui

package main

import (
	"fmt"
	"os"

	"github.com/oslyak/skrynia/vault"
)

func runGUI(_ *vault.Vault, _, _, _ string) {
	fmt.Fprintln(os.Stderr, "error: GUI not available in this build (built with -tags nogui)")
	os.Exit(2)
}
