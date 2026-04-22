# Integrating `skrynia` into your Go project

Guide for developers who want to use the `vault` and `tpmkey` packages as
libraries inside their own Go applications.

> Українська версія: [library-integration.uk.md](./library-integration.uk.md)

## 1. Add the dependency

```bash
cd your-project
go get github.com/oslyak/skrynia@latest
```

Your `go.mod` will gain:

```go
require github.com/oslyak/skrynia v1.0.9
```

You can import **both** packages or **just one** — they are decoupled:

- `github.com/oslyak/skrynia/vault` — high-level store (AES-256-GCM JSON,
  get/set/list/delete/env/export/import). This is what most projects want.
- `github.com/oslyak/skrynia/tpmkey` — low-level TPM seal/unseal of a 32-byte
  key (no JSON store on top).

**Important:** neither package pulls in GTK4. The GUI lives in `cmd/skrynia/`
and is not exported as a library.

## 2. Environment requirements

| Platform | What you need                                             |
|----------|-----------------------------------------------------------|
| Linux    | TPM 2.0 device `/dev/tpmrm0`, user in the `tss` group     |
| Windows  | TPM 2.0 + TBS API (no special privileges needed)          |
| macOS    | No TPM → the library will return an error                 |

Probe before using:

```go
if !tpmkey.Available() {
    log.Fatal("TPM 2.0 required")
}
```

## 3. Typical usage — vault

```go
package main

import (
    "fmt"
    "log"

    "github.com/oslyak/skrynia/vault"
)

func main() {
    // Option A: platform-default location
    //   ~/.local/share/skrynia/vault.{key,dat} on Linux
    //   %APPDATA%\skrynia\vault.{key,dat}      on Windows
    path, err := vault.DefaultPath()
    if err != nil {
        log.Fatal(err)
    }

    // Option B: your own path (pass it without extension — vault appends .key and .dat)
    // path := "/var/lib/myapp/secrets"

    v, err := vault.Open(path)
    if err != nil {
        log.Fatal(err) // TPM unavailable, no access to /dev/tpmrm0, corrupted vault, etc.
    }
    defer v.Close() // ⚠️ mandatory: flushes + zeros the master key in memory

    // Read
    token, err := v.Get("github", "token")
    if err == vault.ErrNotFound {
        fmt.Println("token not set")
    }

    // Write
    if err := v.Set("github", "token", "ghp_xxx..."); err != nil {
        log.Fatal(err)
    }

    // Enumerate
    services, _ := v.List("")           // ["github", "redmine"]
    keys, _     := v.List("github")     // ["login", "token"]

    // Bulk export to env-style
    env, _ := v.Env("github")           // map["LOGIN":..., "TOKEN":...] (UPPER, - → _)
    for k, val := range env {
        fmt.Printf("%s=%s\n", k, val)
    }

    // Backup (encrypted blob with "SKR1" magic)
    blob, _ := v.Export()
    // ... persist blob ...
    // v.Import(blob) — restore
}
```

## 4. Low-level usage — tpmkey

Use this when you **don't** need the JSON store, just a TPM-bound key for your
own crypto:

```go
package main

import "github.com/oslyak/skrynia/tpmkey"

func main() {
    // First run: generate a 32-byte key + seal it under the SRK
    blob, key, err := tpmkey.SealNewKeyRetain()
    if err != nil {
        panic(err)
    }
    defer zero(key) // zero after use
    // ... persist blob to a file ...
    _ = blob

    // Subsequent runs: unseal
    // blob, _ := os.ReadFile("myapp.key")
    // key, err := tpmkey.Unseal(blob)

    // The 32-byte key can now be used directly as an AES-256 key,
    // HKDF seed, HMAC key, etc.
    _ = key
}

func zero(b []byte) {
    for i := range b {
        b[i] = 0
    }
}
```

## 5. Concurrency

`*vault.Vault` is **safe for concurrent use** within a single process (it holds
an internal `sync.Mutex`). All `Get/Set/List/Delete/Env/Export/Import` calls are
serialized.

**However:** a single vault file is not designed for multi-process access —
concurrent writes from two processes will race on `.dat`. If several processes
need the same store, run a single owner process and have others talk to it via
IPC (or via the `skrynia get ...` CLI).

## 6. Error handling

Exported sentinel errors:

```go
vault.ErrNotFound      // service or key does not exist
vault.ErrBadMagic      // Import got a blob without the "SKR1" prefix
vault.ErrBadPayload    // Import could not decrypt/parse the payload
```

Check with `errors.Is`:

```go
if _, err := v.Get("svc", "key"); errors.Is(err, vault.ErrNotFound) {
    // ...
}
```

## 7. Building your project

- **Linux**: `CGO_ENABLED=0` — the `vault` and `tpmkey` packages do **not**
  require CGO. Pure Go, static linking works.
- **Windows**: same — pure syscall.
- If your binary uses CGO for other reasons, that's still fine.

```bash
CGO_ENABLED=0 go build -o myapp ./cmd/myapp
```

## 8. Storage constraints and gotchas

1. **The key is hardware-bound.** `vault.dat` + `vault.key` from machine A
   **cannot** be decrypted on machine B, even on the same OS with the same
   user. This is by design — a feature, not a bug. For migration use
   `Export()` → transfer → `Import()`.

2. **Deleting `vault.key` = losing the data.** No recovery mechanism exists.

3. **Linux without the `tss` group**: `vault.Open` returns
   `TPM 2.0 not available`. Fix: `sudo usermod -aG tss $USER` + re-login, or
   run via `sg tss -c "...".`

4. **Docker / containers**: pass `/dev/tpmrm0` into the container
   (`--device=/dev/tpmrm0`), otherwise the TPM is invisible. For dev
   environments without a TPM it's usually easier to read credentials via
   the host-side `skrynia get ...` CLI.

5. **Always `defer v.Close()`.** Without it the last change may not be flushed
   and the master key will remain in process memory.

## 9. Minimal `go.mod` example

```go
module example.com/myapp

go 1.25

require github.com/oslyak/skrynia v1.0.9
```

That's it — from here `vault.Open()` behaves like any local store.

## Public API — quick reference

### Package `vault`

| Symbol                                             | Description                                            |
|----------------------------------------------------|--------------------------------------------------------|
| `DefaultPath() (string, error)`                    | Platform-default base path                             |
| `Open(basePath string) (*Vault, error)`            | Open or create the store                               |
| `(*Vault).Close() error`                           | Flush + zero the key                                   |
| `(*Vault).Get(service, key) (string, error)`       | Read a value                                           |
| `(*Vault).Set(service, key, value) error`          | Write a value                                          |
| `(*Vault).List(service) ([]string, error)`         | `""` → all services; otherwise → keys of the service   |
| `(*Vault).Delete(service, key) error`              | `key=""` → delete the whole service                    |
| `(*Vault).Env(service) (map[string]string, error)` | KEY=VALUE with normalization (UPPER, `-` → `_`)        |
| `(*Vault).Export() ([]byte, error)`                | Encrypted `SKR1` blob                                  |
| `(*Vault).Import(blob []byte) error`               | Merge from blob                                        |
| `ErrNotFound`, `ErrBadMagic`, `ErrBadPayload`      | Sentinel errors                                        |

### Package `tpmkey`

| Symbol                                             | Description                                            |
|----------------------------------------------------|--------------------------------------------------------|
| `Available() bool`                                 | Whether the TPM is reachable (no error, just bool)     |
| `SealNewKey() ([]byte, error)`                     | Generate + seal a key, return only the blob            |
| `SealNewKeyRetain() ([]byte, []byte, error)`       | Same + return the plaintext key (caller zeros it)      |
| `Unseal(blob []byte) ([]byte, error)`              | Unseal the key back                                    |
