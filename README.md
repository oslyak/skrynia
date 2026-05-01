# Skrynia

> **TPM-bound credentials your AI agent can't see.**

![demo](docs/demo.gif)

A credential manager built for the age of AI-assisted development.

**The problem:** When you work with AI coding agents (Claude Code, Cursor, Codex, Copilot CLI), every command you type is seen by the agent — including passwords and API keys. Credentials leak into:

- Shell history (`~/.bash_history`)
- AI conversation context sent to LLM providers
- AI agent logs and session transcripts
- `.env` files that supply-chain malware scans for

**The solution:** Skrynia acts as a **gateway between your AI agent and your secrets**. The agent can *use* credentials without ever *seeing* them.

```bash
# You tell the agent:
curl -u "$(skrynia get redmine login):$(skrynia get redmine password)" https://...

# The agent runs the command — values appear only in the subprocess,
# never in the agent's context, never in shell history.
```

Credentials are stored in an AES-256-GCM encrypted vault, with the master key sealed by your machine's TPM 2.0 chip. Even if an attacker copies `vault.dat` and `vault.key`, they're useless without the physical TPM.

**skrynia** (скриня) is Ukrainian for "chest" — a secure container for valuables.

## Features

- **AI-agent-safe writes** — passwords typed in a GUI dialog, bypassing terminal → never in shell history, never in AI conversation context
- **AI-agent-safe reads** — `$(skrynia get redmine password)` via command substitution → agent sees the command, not the value
- **TPM 2.0 hardware binding** — master key sealed by machine's TPM chip; vault files are useless on any other machine
- **AES-256-GCM** — authenticated encryption at rest, not XOR or homebrew crypto
- **Smart key detection** — `password`, `token`, `secret`, `api-key` auto-open GUI; non-sensitive values stay in CLI
- **Supply-chain resistant** — no `.env` files, no `*credential*` patterns for malware to scan
- **Cross-platform** — Linux (GTK4) + Windows (Win32 API); macOS not supported (no TPM 2.0 — Apple uses Secure Enclave)
- **Headless variant** — `skrynia-cli` binary (built with `-tags nogui`) for servers, containers, and CI
- **Reusable Go packages** — `vault` and `tpmkey` can be imported by other projects as standalone libraries

## Requirements

- **TPM 2.0** hardware (no fallback — by design)
- **Linux**: user must be in `tss` group, device `/dev/tpmrm0`, GTK4
- **Windows**: TBS API, no special permissions needed
- **macOS**: not supported — Apple devices have no TPM 2.0 (they use Secure Enclave, which `go-tpm` cannot access)
- Go 1.25+ (for building)

## Installation

```bash
make build    # builds skrynia (linux GUI), skrynia-cli (linux nogui), skrynia.exe (windows)
make install  # copies all three binaries to ~/ai/bin/
```

## Usage

### Reading credentials (CLI, stdout)

```bash
skrynia get <service> <key>

# Examples:
$(skrynia get redmine login)        # → username
$(skrynia get redmine password)     # → ****
$(skrynia get redmine api-key)      # → 5a5ef10f18...

# In scripts:
curl -u "$(skrynia get redmine login):$(skrynia get redmine password)" https://rm.example.com/...
```

### Writing credentials (GUI)

```bash
skrynia set redmine credentials       # GUI: Login + Password
skrynia set gateway api-key           # GUI: API Key (masked)
skrynia set redmine password          # GUI auto-opens (sensitive key detected)
skrynia set redmine token             # GUI auto-opens (sensitive key detected)
```

### Writing non-sensitive values (CLI)

```bash
skrynia set redmine url "https://rm.example.com"
skrynia set redmine custom-field "any value"
```

### Overriding defaults

```bash
skrynia set redmine url --gui            # force GUI for non-sensitive key
skrynia set redmine password "val" --cli # force CLI for sensitive key
```

### Other commands

```bash
skrynia list redmine               # list keys for a service
skrynia delete redmine password    # delete a key
skrynia delete redmine             # delete entire service
skrynia env redmine                # print normalized KEY=VALUE pairs (UPPERCASE, - → _)
skrynia export > backup.enc        # encrypted backup to stdout
skrynia import < backup.enc        # restore from stdin
skrynia --version                  # print version
```

### Values that start with `--`

Use the `--` separator to pass positional values that look like flags:

```bash
skrynia set myservice custom-flag -- --cli    # stored value is literally "--cli"
```

## Architecture

```
Reading:  skrynia get <service> <key>               →  stdout (CLI)
Writing:  skrynia set <service> credentials         →  GUI (login + password)
          skrynia set <service> api-key             →  GUI (masked input)
          skrynia set <service> <sensitive>         →  GUI (auto-detected)
          skrynia set <service> <key> <value>       →  CLI (non-sensitive)
```

### Project structure

Skrynia is split into **two reusable library packages** and **one application binary**:

```
skrynia/
├── vault/        ← library: AES-256-GCM encrypted JSON store (get/set/list/delete/env/export/import)
├── tpmkey/       ← library: TPM 2.0 seal/unseal of the 32-byte master key
└── cmd/skrynia/  ← application: CLI + platform GUI (GTK4 on Linux, Win32 on Windows)
```

`vault` depends on `tpmkey`; both are free of GUI code and safe to import from other
Go projects that need TPM-backed encrypted storage without the skrynia CLI itself.

### GUI by platform

| Platform | Technology | Dependencies | Binary size |
|----------|-----------|-------------|-------------|
| Linux    | GTK4 (gotk4) | libgtk-4 | ~12 MB |
| Windows  | Win32 API (syscall) | none | ~3.5 MB |

### Sensitive key auto-detection

Keys containing these words automatically open GUI: `password`, `passwd`, `secret`, `token`, `api-key`, `private-key`, `credential`.

### Encryption flow

1. **First run**: generate 32-byte random key → seal with TPM SRK (ECC-P256) → store in `vault.key`
2. **Each run**: unseal `vault.key` via TPM → AES-256-GCM decrypt `vault.dat` → work → encrypt → save
3. **On close**: encrypt data, write `vault.dat`, zero master key in memory

### Storage

| Component  | Linux                                | Windows                       |
|------------|--------------------------------------|-------------------------------|
| Sealed key | `~/.local/share/skrynia/vault.key`   | `%APPDATA%\skrynia\vault.key` |
| Vault data | `~/.local/share/skrynia/vault.dat`   | `%APPDATA%\skrynia\vault.dat` |

- Key is TPM-sealed on first run (32 random bytes → sealed blob)
- Vault is AES-256-GCM encrypted JSON
- No key file = credentials are lost (by design, not a bug)
- Key can only be unsealed on the same physical machine

### Using as a Go library

Either package can be imported independently. `vault` is the high-level API most
projects will want; `tpmkey` is useful if you need raw TPM seal/unseal without the
JSON store on top.

```go
import (
    "github.com/oslyak/skrynia/vault"
    "github.com/oslyak/skrynia/tpmkey"
)

// Use the platform-default location, or pass your own base path.
path, _ := vault.DefaultPath()
v, err := vault.Open(path)
if err != nil {
    // TPM unavailable, user not in 'tss' group on Linux, etc.
    return err
}
defer v.Close() // encrypts, flushes to disk, zeroes master key in memory

password, err := v.Get("redmine", "password")
_ = v.Set("redmine", "url", "https://rm.example.com")
services, _ := v.List("")           // all service names
keys, _     := v.List("redmine")    // all keys of a service
env, _      := v.Env("redmine")     // {"LOGIN": "...", "PASSWORD": "..."} (keys normalized)
blob, _     := v.Export()           // encrypted SKR1 backup

// Lower-level TPM access (no JSON store):
blob, key, err := tpmkey.SealNewKeyRetain() // 32-byte key sealed under TPM SRK
key2, err      := tpmkey.Unseal(blob)
available      := tpmkey.Available()         // probe without raising errors
```

Both packages follow the contract: **no TPM → no operation**. There is no in-memory
fallback, and `vault.Open` will return an error if `tpmkey.Available()` is false.

📖 **Full integration guide** with concurrency, error handling, Docker notes, and API reference:
[docs/library-integration.md](docs/library-integration.md) (English) ·
[docs/library-integration.uk.md](docs/library-integration.uk.md) (Ukrainian)

### Export format

```
[4 bytes magic: "SKR1"]
[N bytes AES-GCM encrypted JSON payload]
```

## Build

```bash
make build              # all three binaries
make build-linux        # linux/amd64 GUI (CGO_ENABLED=1, needs GTK4 dev)
make build-linux-nogui  # linux/amd64 CLI-only (CGO_ENABLED=0, -tags nogui)
make build-windows      # windows/amd64 (CGO_ENABLED=0, pure syscall)
make test               # run all tests (requires TPM access; uses `sg tss`)
make install            # copy all binaries to ~/ai/bin/
make bump               # increment patch version in VERSION file
make clean              # remove build/bin/*
```

### Build outputs

| Binary             | Platform | GUI   | Use case                                 |
|--------------------|----------|-------|------------------------------------------|
| `skrynia`          | Linux    | GTK4  | Desktop workstation                      |
| `skrynia-cli`      | Linux    | none  | Servers, containers, CI (set-with-value) |
| `skrynia.exe`      | Windows  | Win32 | Windows desktop                          |

The `-tags nogui` build has zero GTK dependency and statically-linked Go runtime —
ideal for headless hosts where credentials are provisioned via `set <svc> <key> <val>`
or `import < backup.enc`.

### Build dependencies

**Linux (GUI)**: `apt install libgtk-4-dev libgirepository1.0-dev`

**Linux (CLI-only)**: no dependencies beyond the Go toolchain

**Windows cross-compile from Linux**: pure `CGO_ENABLED=0`, no MinGW needed

## ⚠️ Important: Auxiliary Storage Only

Skrynia is **hardware-bound**: the TPM chip seals the master key to a specific motherboard. If your motherboard dies, is replaced, or the TPM is reset — **all stored credentials become permanently unrecoverable**. There is no recovery mechanism by design.

**Do not keep sensitive data only in skrynia.** Always maintain a primary backup elsewhere (password manager, printed sheet in a safe, encrypted USB drive). Treat skrynia as an **auxiliary convenience store** for day-to-day use, not as your sole vault.

## Name

**Skrynia** (скриня, /ˈskrɪnʲɑ/) means "chest" or "box" in Ukrainian — a sturdy container where valuables are kept safe.

## License

MIT
