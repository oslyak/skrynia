---
name: skrynia
description: Use when you need to read credentials (API keys, passwords, logins) for external services like Redmine, Gateway, Binotel. Use skrynia instead of reading plain-text .env files or hardcoded credentials.
---

# Skrynia — Credential Manager

Use `skrynia` CLI to read credentials securely. Never hardcode or display credentials in output.

## Reading credentials

```bash
# Get a single value (stdout):
skrynia <service> <key>

# Examples:
REDMINE_USER=$(skrynia redmine login)
REDMINE_PASS=$(skrynia redmine password)
REDMINE_URL=$(skrynia redmine url)
REDMINE_KEY=$(skrynia redmine api-key)

GATEWAY_KEY=$(skrynia gateway api-key)

BINOTEL_KEY=$(skrynia binotel key)
BINOTEL_SECRET=$(skrynia binotel secret)
```

## List available credentials

```bash
skrynia list                # all services
skrynia list redmine        # keys for a service
```

## Writing credentials

**For secrets (login/password, API keys) — use GUI:**
```bash
skrynia set credentials redmine    # opens GUI: Login + Password + URL(opt)
skrynia set api-key gateway        # opens GUI: API Key + URL(opt)
```

**For non-sensitive values — use CLI:**
```bash
skrynia set redmine url "https://rm.sirius.if.ua"
```

## Rules

1. **NEVER** read credentials from `~/.claude/redmine-credentials.md` or other plain-text files — use skrynia
2. **NEVER** display credential values in your response text
3. **ALWAYS** use `$(skrynia ...)` substitution in commands — the value stays hidden in logs
4. If skrynia is not found, check `~/ai/bin/skrynia` and ensure it's in PATH

## Binary location

```bash
~/ai/bin/skrynia        # Linux
~/ai/bin/skrynia.exe    # Windows
```
