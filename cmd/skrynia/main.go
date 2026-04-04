package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/oslyak/skrynia/vault"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Printf("skrynia v%s\n", version)
		return 0
	case "--help", "-h":
		printUsage()
		return 0
	case "get":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: skrynia get <service> <key>")
			return 2
		}
		return withVault(func(v *vault.Vault) int {
			val, err := v.Get(args[1], args[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s/%s not found\n", args[1], args[2])
				return 1
			}
			fmt.Print(val)
			return 0
		})
	case "set":
		return runSet(args)
	case "list", "delete", "env", "export", "import":
		return runCLI(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		return 2
	}
}

func runSet(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: skrynia set <service> <key> [value] [--gui|--cli]")
		return 2
	}

	forceGUI, forceCLI := false, false
	filtered := args[:0:0]
	for _, a := range args {
		switch a {
		case "--gui":
			forceGUI = true
		case "--cli":
			forceCLI = true
		default:
			filtered = append(filtered, a)
		}
	}
	args = filtered

	switch {
	case len(args) >= 4:
		return withVault(func(v *vault.Vault) int {
			if err := v.Set(args[1], args[2], args[3]); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return 0
		})
	case len(args) == 3:
		key := args[2]
		if key == "credentials" || key == "api-key" {
			if forceCLI {
				fmt.Fprintf(os.Stderr, "error: %s requires GUI, cannot use --cli\n", key)
				return 2
			}
			return withVault(func(v *vault.Vault) int {
				runGUI(v, args[1], key, detectLang())
				return 0
			})
		} else if forceGUI || (!forceCLI && isSensitiveKey(key)) {
			return withVault(func(v *vault.Vault) int {
				runGUI(v, args[1], key, detectLang())
				return 0
			})
		} else if forceCLI {
			fmt.Fprintln(os.Stderr, "error: --cli requires a value: skrynia set <service> <key> <value> --cli")
			return 2
		} else {
			fmt.Fprintf(os.Stderr, "usage: skrynia set %s %s <value>  or  skrynia set %s %s --gui\n", args[1], key, args[1], key)
			return 2
		}
	case len(args) == 2:
		return withVault(func(v *vault.Vault) int {
			runGUI(v, args[1], "credentials", detectLang())
			return 0
		})
	}
	return 0
}

// withVault opens the vault, runs fn, and guarantees Close() + key zeroing.
func withVault(fn func(v *vault.Vault) int) int {
	dbPath, err := vault.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	v, err := vault.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer v.Close()
	return fn(v)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "skrynia v%s\n\n", version)
	fmt.Fprintln(os.Stderr, `Usage:
  skrynia get <service> <key>              Read credential (stdout)
  skrynia set <service> credentials        Set login/password via GUI
  skrynia set <service> api-key            Set API key via GUI
  skrynia set <service> <key> <value>      Set value via CLI
  skrynia set <service> <key>              Auto: GUI if sensitive, error otherwise
  skrynia set <service> <key> --gui        Force GUI for any key
  skrynia set <service> <key> <val> --cli  Force CLI (skip GUI even for sensitive)
  skrynia list <service>                   List keys of a service
  skrynia delete <service> [key]           Delete service or key
  skrynia env <service>                    Print KEY=VALUE pairs
  skrynia export                           Export encrypted backup
  skrynia import                           Import encrypted backup
  skrynia --version                        Print version

Sensitive keys (auto-GUI): password, secret, token, api-key, private-key

Examples:
  $(skrynia get redmine password)          Use in scripts
  skrynia set redmine credentials          Open GUI for login/password
  skrynia set redmine password             Auto-opens GUI (sensitive key)
  skrynia set redmine password val --cli   Force CLI for sensitive key
  skrynia set redmine url "https://..."    Set non-sensitive value via CLI
  skrynia set redmine url --gui            Force GUI for non-sensitive key`)
}

func runCLI(args []string) int {
	switch args[0] {
	case "list":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: skrynia list <service>")
			return 2
		}
		return withVault(func(v *vault.Vault) int {
			items, err := v.List(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			for _, item := range items {
				fmt.Println(item)
			}
			return 0
		})

	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: skrynia delete <service> [key]")
			return 2
		}
		return withVault(func(v *vault.Vault) int {
			key := ""
			if len(args) > 2 {
				key = args[2]
			}
			if err := v.Delete(args[1], key); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return 0
		})

	case "env":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: skrynia env <service>")
			return 2
		}
		return withVault(func(v *vault.Vault) int {
			env, err := v.Env(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			keys := make([]string, 0, len(env))
			for k := range env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s=%s\n", k, env[k])
			}
			return 0
		})

	case "export":
		return withVault(func(v *vault.Vault) int {
			blob, err := v.Export()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			os.Stdout.Write(blob)
			return 0
		})

	case "import":
		return withVault(func(v *vault.Vault) int {
			blob, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if err := v.Import(blob); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return 0
		})

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		return 2
	}
}

// isSensitiveKey returns true if the key name suggests a sensitive value
// that should be entered via GUI by default (not visible in shell history).
func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.ReplaceAll(key, "-", ""))
	k = strings.ReplaceAll(k, "_", "")
	sensitive := []string{"password", "passwd", "secret", "token", "apikey", "privatekey", "credential"}
	for _, s := range sensitive {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func detectLang() string {
	for _, env := range []string{"LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"} {
		val := os.Getenv(env)
		if val == "" {
			continue
		}
		if strings.HasPrefix(val, "uk") || strings.HasPrefix(val, "ru") {
			return "uk"
		}
		return "en"
	}
	return "en"
}
