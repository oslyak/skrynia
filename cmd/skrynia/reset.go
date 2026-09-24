package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/oslyak/skrynia/vault"
)

func runReset(args []string) int {
	flags := flag.NewFlagSet("reset", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "explicitly confirm starting with an empty vault")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: skrynia reset [--yes]")
		return 2
	}
	path, err := vault.DefaultPath()
	if err != nil {
		return reportError(err)
	}
	fmt.Fprintln(os.Stderr, "Reset creates an EMPTY vault on the current TPM. Old encrypted files will be archived; old passwords will not be recovered.")
	if !*yes {
		tty, err := openTerminal()
		if err != nil {
			return reportError(fmt.Errorf("confirmation requires a terminal; use --yes only to explicitly confirm reset"))
		}
		defer tty.Close()
		if !confirmReset(tty, os.Stderr) {
			fmt.Fprintln(os.Stderr, "Reset cancelled; vault unchanged.")
			return 1
		}
	}
	archive, err := vault.Reset(path)
	if archive != "" {
		fmt.Fprintf(os.Stderr, "Archive: %s\n", archive)
	}
	if err != nil {
		return reportError(err)
	}
	fmt.Fprintln(os.Stderr, "New empty vault is ready. Use skrynia set to add credentials.")
	return 0
}

func confirmReset(input io.Reader, output io.Writer) bool {
	fmt.Fprint(output, "Type RESET to continue: ")
	answer, err := bufio.NewReader(input).ReadString('\n')
	return err == nil && strings.TrimSpace(answer) == "RESET"
}
