# Skrynia demo — TPM-bound credentials your AI agent can't see
# Regenerate: vhs docs/demo.tape

Output docs/demo.gif

Set FontSize 16
Set Width 1100
Set Height 620
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 50ms
Set PlaybackSpeed 0.9

# Use an isolated demo vault — main vault is untouched
Hide
Type "export HOME=/tmp/skrynia-demo"
Enter
Type "rm -rf $HOME && mkdir -p $HOME && cd $HOME"
Enter
Type "clear"
Enter
Show
Sleep 600ms

Type "# Skrynia: TPM-sealed credentials your AI agent can't see"
Enter
Sleep 1500ms

# Store credentials (in production, sensitive keys auto-open a GUI dialog)
Type "skrynia set demo login 'hopper'"
Enter
Sleep 700ms
Type "skrynia set demo password 'sup3r-s3cr3t' --cli"
Enter
Sleep 700ms

# Real-world use: API call with basic auth — value lives only in the subprocess
Type `curl -su "$(skrynia get demo login):$(skrynia get demo password)" \`
Enter
Type "  https://httpbin.org/basic-auth/hopper/sup3r-s3cr3t"
Enter
Sleep 5000ms

Type "# Secret never touches .env, history, or your AI agent's context."
Enter
Sleep 2200ms

# Show the vault on disk is encrypted garbage
Type "# Vault on disk: AES-256-GCM, master key sealed by TPM 2.0"
Enter
Sleep 1500ms

Type "cd $HOME/.local/share/skrynia/ && ls"
Enter
Sleep 1800ms

Type "od -An -tx1 -N 48 vault.dat"
Enter
Sleep 3000ms

Type "# Without this machine's TPM — vault.dat is useless."
Enter
Sleep 3000ms

Hide
Type "rm -rf /tmp/skrynia-demo"
Enter
Show
