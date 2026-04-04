# Skrynia - Cross-Platform Credential Manager

## TPM 2.0 Encryption
- **Requirement**: TPM 2.0 hardware is mandatory — no fallback
- **Linux**: user must be in `tss` group, device `/dev/tpmrm0`
- **Windows**: TBS API, no special permissions needed
- **Master key**: 32-byte random key, sealed under TPM's SRK (ECC-P256)
- **Storage**: sealed blob stored in `vault.key` file
- **Library**: `github.com/google/go-tpm` v0.9.8

## Vault Storage
- **Format**: JSON file encrypted with AES-256-GCM using TPM-sealed master key
- **Files**:
  - `vault.key` — TPM-sealed master key blob
  - `vault.dat` — AES-GCM encrypted JSON (`{service: {key: value, ...}, ...}`)
- **Linux**: `~/.local/share/skrynia/vault.key` + `vault.dat`
- **Windows**: `%APPDATA%\skrynia\vault.key` + `vault.dat`
- **No SQLite, no XOR** — pure AES-GCM with TPM-sealed key

## Build
- **Linux**: `CGO_ENABLED=1` (GTK4 via gotk4 requires CGO)
- **Windows**: `CGO_ENABLED=0` (pure Win32 API via syscall)
- **Build**: `make build` (builds linux + windows)
- **Test**: `sg tss -c "go test ./..."` (needs tss group for TPM access)
- **Install**: `make install` (copies to `~/ai/bin/`)

## GUI by Platform
- **Linux/macOS**: GTK4 via `github.com/diamondburned/gotk4`
- **Windows**: Win32 API via `syscall` (no external dependencies)

## CLI vs GUI Dispatch
- `skrynia get <service> <key>` — CLI read (no GUI)
- `skrynia set <service> credentials` — GUI (login + password)
- `skrynia set <service> api-key` — GUI (masked input)
- `skrynia set <service> <sensitive-key>` — GUI (auto-detected)
- `skrynia set <service> <key> <value>` — CLI set (non-sensitive)
- `skrynia set <service> <key> --gui` — force GUI
- `skrynia set <service> <key> <value> --cli` — force CLI
- `skrynia list/delete/env/export/import` — CLI
- `skrynia --version` — CLI
- Sensitive keys: password, passwd, secret, token, apikey, privatekey, credential

## Encryption Flow
1. First run: `SealNewKeyRetain()` → 32-byte random key sealed by TPM → stored in `vault.key`
2. Each run: `Unseal(blob)` → 32-byte master key → AES-256-GCM encrypt/decrypt `vault.dat`
3. Export format: magic `SKR1` (4 bytes) + AES-GCM encrypted JSON

## Testing
- `github.com/stretchr/testify` with `require := require.New(t)`
- Table-driven tests use `tc` as loop variable
- Tests require TPM access — NO t.Skip() when TPM unavailable
