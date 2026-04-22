---
name: skrynia
description: Use when you need to read credentials (API keys, passwords, logins) for external services like Redmine, Gateway, Binotel. Use skrynia instead of reading plain-text .env files or hardcoded credentials.
---

# Skrynia — Credential Manager

Use `skrynia` CLI to read credentials securely. Never hardcode or display credentials in output.

## Reading credentials

```bash
# Get a single value (stdout):
skrynia get <service> <key>

# Examples:
REDMINE_USER=$(skrynia get redmine login)
REDMINE_PASS=$(skrynia get redmine password)
REDMINE_URL=$(skrynia get redmine url)
REDMINE_KEY=$(skrynia get redmine api-key)

GATEWAY_KEY=$(skrynia get gateway api-key)

BINOTEL_KEY=$(skrynia get binotel key)
BINOTEL_SECRET=$(skrynia get binotel secret)

# All key=value pairs for a service (normalized: UPPERCASE, - → _):
eval "$(skrynia env redmine)"   # sets LOGIN, PASSWORD, URL, API_KEY, ...
```

## Listing available credentials

```bash
skrynia list redmine        # keys for a specific service (service arg is required)
```

## Writing credentials

**For secrets (login/password, API keys) — GUI opens automatically:**

```bash
skrynia set redmine credentials    # GUI: Login + Password
skrynia set gateway api-key        # GUI: masked API Key input
skrynia set redmine password       # GUI auto-opens (sensitive key detected)
skrynia set redmine token          # GUI auto-opens (sensitive key detected)
```

Sensitive key substrings auto-open GUI: `password`, `passwd`, `secret`, `token`, `apikey`, `privatekey`, `credential`.

**For non-sensitive values — CLI (pass the value as the last arg):**

```bash
skrynia set redmine url "https://rm.sirius.if.ua"
skrynia set redmine custom-field "any value"
```

**Overrides:**

```bash
skrynia set redmine url --gui              # force GUI for non-sensitive key
skrynia set redmine password "x" --cli     # force CLI for sensitive key (value required)
skrynia set svc key -- --cli               # -- makes "--cli" a literal value
```

## Other commands

```bash
skrynia delete redmine password    # delete one key
skrynia delete redmine             # delete entire service
skrynia env redmine                # print KEY=VALUE pairs (UPPERCASE, - → _)
skrynia export > backup.enc        # encrypted backup to stdout
skrynia import < backup.enc        # restore from stdin
skrynia --version                  # print version
```

## Rules

1. **NEVER** read credentials from plain-text `.env` files or hardcoded values — use skrynia
2. **NEVER** display credential values in your response text
3. **ALWAYS** use `$(skrynia get ...)` substitution in commands — the value stays hidden in the agent context and shell history
4. If skrynia is not found, check `~/ai/bin/skrynia` and ensure it's in `$PATH`
5. On headless Linux (no GTK), use `skrynia-cli` — same CLI, no GUI; set sensitive values with `set <svc> <key> <val> --cli` or provision via `import < backup.enc`

## Binary locations

```bash
~/ai/bin/skrynia         # Linux (GUI)
~/ai/bin/skrynia-cli     # Linux (headless, -tags nogui)
~/ai/bin/skrynia.exe     # Windows
```
