# Skrynia

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
- **Cross-platform** — Linux (GTK4) + Windows (Win32 API), macOS (via GTK4)
- **Reusable Go packages** — `vault` and `tpmkey` can be imported by other projects

## Requirements

- **TPM 2.0** hardware (no fallback — by design)
- **Linux**: user must be in `tss` group, device `/dev/tpmrm0`, GTK4
- **Windows**: TBS API, no special permissions needed
- **macOS**: `brew install gtk4 gobject-introspection`
- Go 1.25+ (for building)

## Installation

```bash
make build    # builds linux + windows binaries
make install  # copies to ~/ai/bin/
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
skrynia env redmine                # print KEY=VALUE pairs
skrynia export > backup.enc        # encrypted backup
skrynia import < backup.enc        # restore from backup
skrynia --version                  # print version
```

## Architecture

```
Reading:  skrynia get <s> <k>           →  stdout (CLI)
Writing:  skrynia set <s> credentials   →  GUI (login + password)
          skrynia set <s> api-key       →  GUI (masked input)
          skrynia set <s> <sensitive>   →  GUI (auto-detected)
          skrynia set <s> <k> <v>       →  CLI (non-sensitive)
```

### GUI by platform

| Platform | Technology | Dependencies | Binary size |
|----------|-----------|-------------|-------------|
| Linux    | GTK4 (gotk4) | libgtk-4 | ~12 MB |
| Windows  | Win32 API (syscall) | none | ~3.5 MB |
| macOS    | GTK4 (gotk4) | brew gtk4 | ~12 MB |

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

```go
import (
    "github.com/<user>/skrynia/vault"
    "github.com/<user>/skrynia/tpmkey"
)

v, err := vault.Open("/path/to/vault")
defer v.Close()

password, err := v.Get("redmine", "password")
```

### Export format

```
[4 bytes magic: "SKR1"]
[N bytes AES-GCM encrypted JSON payload]
```

## Build

```bash
make build          # both platforms
make build-linux    # linux/amd64 (CGO_ENABLED=1, needs GTK4 dev)
make build-windows  # windows/amd64 (CGO_ENABLED=0)
make test           # run all tests (requires TPM access)
make install        # copy to ~/ai/bin/
```

### Build dependencies

**Linux**: `apt install libgtk-4-dev libgirepository1.0-dev`

**Windows cross-compile**: `apt install gcc-mingw-w64-x86-64` (CGO not needed)

## ⚠️ Important: Auxiliary Storage Only

Skrynia is **hardware-bound**: the TPM chip seals the master key to a specific motherboard. If your motherboard dies, is replaced, or the TPM is reset — **all stored credentials become permanently unrecoverable**. There is no recovery mechanism by design.

**Do not keep sensitive data only in skrynia.** Always maintain a primary backup elsewhere (password manager, printed sheet in a safe, encrypted USB drive). Treat skrynia as an **auxiliary convenience store** for day-to-day use, not as your sole vault.

## Name

**Skrynia** (скриня, /ˈskrɪnʲɑ/) means "chest" or "box" in Ukrainian — a sturdy container where valuables are kept safe.

## License

MIT
