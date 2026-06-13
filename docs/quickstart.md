# Quick Start Guide

> Get Shadow running in under 5 minutes.
> By the end, you'll have a rule synced across all your coding agents.

## Prerequisites

- **macOS** (Apple Silicon or Intel) or **Linux** (x86_64 or arm64)
- A terminal with color support
- At least one coding agent installed:
  - [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (recommended for first try)
  - [Cursor](https://cursor.sh)
  - GitHub Copilot CLI
  - Codex CLI

## Step 1: Install Shadow

### Option A: Homebrew (recommended)

```bash
brew tap joevilcai666/shadow
brew install --formula joevilcai666/shadow/shadow
```

Verify:

```bash
shadow version
# Shadow 0.3.0
```

### Option B: Binary download

1. Go to [GitHub Releases](https://github.com/joevilcai666/shadow/releases)
2. Download the archive for your platform
3. Extract and move to your PATH:

```bash
# macOS Apple Silicon example
tar xzf shadow_0.3.0_darwin_arm64.tar.gz
sudo mv shadow /usr/local/bin/
```

### Option C: Build from source

You need Go 1.25+ and Node.js 22+.

```bash
git clone https://github.com/joevilcai666/shadow.git
cd shadow
make build
sudo make install
```

## Step 2: Start Shadow

Navigate to a project directory where you use a coding agent:

```bash
cd ~/your-project   # any Git repo
shadow start
```

### What you'll see

**Screen 1 — Welcome**

```
 ██████  ██   ██  █████   ██████  ██████   ██████  ██     ██
██       ██   ██ ██   ██ ██    ██ ██   ██ ██    ██ ██     ██
███████  ███████ ███████ ██    ██ ██   ██ ██    ██ ██  █  ██
     ██ ██   ██ ██   ██ ██    ██ ██   ██ ██    ██ ██ ███ ██
 ██████ ██   ██ ██   ██  ██████  ██████   ██████   ███ ███

  correct once, remember everywhere.

✓━━━━━○━━━━━○━━━━━○
Setup  Privacy Agents Memory

Welcome to Shadow!

This will take about 60 seconds. Everything stays local.
You can skip, go back, or quit at any time.

Your data:
  ✓ Stored only on this machine
  ✓ Never uploaded without your consent
  ✓ Keys/tokens automatically blocked

Press Enter to begin...
```

Press **Enter**.

**Screen 2 — Privacy & Scope**

Shadow tells you exactly what it will read and write.
All safe defaults. Press **Enter** to accept.

```
Privacy & Scope

Shadow will:
  ✓ Read project code & agent session logs
  ✓ Write managed rules to agent context files
  ✓ Never store keys, tokens, or credentials
  ✓ Store only distilled rules, not raw conversations

🔒 Default protections (always on):
  • Block: API keys, tokens, .env files
  • Exclude: node_modules, .git, dist, .env*
  • Only distilled rules stored, never raw code

Press Enter to accept safe defaults...
```

**Screen 3 — Agent Selection**

Shadow auto-detects installed agents.
Use **Space** to toggle, **↑↓** to move, **Enter** to confirm.

```
Select Agents

Detected 2 agent(s) on your machine:

  > ● Claude Code  — writes to CLAUDE.md
    ● Cursor       — writes to .cursorrules
    ○ GitHub Copilot — writes to .github/copilot-instructions.md
    ○ Codex        — writes to AGENTS.md

↑↓ move · Space toggle · Enter confirm
```

**Screen 4 — Initial Memory Generation**

Shadow scans your project for conventions and existing rules.

```
Initial Memory Generation

Shadow will scan your project for:
  • Package manager (lockfile detection)
  • Test framework (config detection)
  • Existing rules (CLAUDE.md, .cursorrules, AGENTS.md)
  • Language and framework detection

Press Enter to scan...  (s to skip)
```

**Screen 5 — Done!**

```
✓ Shadow is ready!

  Agents connected: Claude Code, Cursor
  Initial memories: 7 candidate rules generated
  Imported from: CLAUDE.md, .cursorrules

Next steps:
  shadow status  — check everything is running
  shadow open    — open web console at localhost:7878
  shadow review  — review candidate rules

Press Enter to open web console (or q to stay in terminal)...
```

## Step 3: Review Your Rules

Candidate rules are **inactive** until you approve them. They won't affect your agents.

### In the terminal

```bash
shadow review
```

Interactive TUI:

| Key | Action |
|-----|--------|
| `a` | Approve current rule |
| `r` | Reject current rule |
| `A` | Approve all rules |
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Apply decisions |
| `q` | Quit without changes |

### In the web console

```bash
shadow open
```

Navigate to the **Review** page in the sidebar. Approve or reject rules
individually or in batch.

## Step 4: Verify It's Working

### Check daemon status

```bash
shadow status
```

Should show the daemon running with state `capturing`.

### Check agent context files

After approving rules, verify Shadow wrote managed blocks:

```bash
cat CLAUDE.md
```

Look for the managed block markers:

```
# >>> shadow managed >>>
# ...
# <<< shadow managed <<<
```

### Test the core loop: correct → rule

1. Open your coding agent (e.g., Claude Code) in the project
2. Let it make a suggestion you disagree with
3. Correct it with a clear instruction like:
   > "No, always use pnpm in this project"
4. Wait ~60 seconds for Shadow's capture engine to process the correction
5. Check for new candidate rules:
   ```bash
   shadow review
   ```
6. Approve the rule
7. Verify it appears in your agent context files

This is the core value: **correct once, every agent remembers**.

## Daily Usage

Once set up, Shadow runs silently in the background. The daemon starts
automatically via launchd on macOS.

```bash
shadow status    # Check it's running
shadow open      # Open the web console anytime
shadow review    # Review new candidate rules
shadow stop      # Pause the daemon
shadow start     # Resume (if stopped)
```

## Web Console

Open `http://localhost:7878` for the full management UI:

| Page | What it does |
|------|-------------|
| **Memory Map** | Visualize your rule graph and relationships |
| **Rules** | Search, filter, create, edit, and delete rules |
| **Review** | Approve or reject candidate rules |
| **Conflicts** | Resolve conflicting rules |
| **Projects** | View registered projects and their agents |
| **Settings** | Toggle adapters, configure capture and privacy |

## MCP Integration

Shadow exposes an MCP server so other tools can query your rules:

```bash
shadow mcp
```

This prints the JSON config to add Shadow as an MCP server in Claude
Desktop, Continue, or any MCP-compatible host.

## Stopping and Uninstalling

### Stop the daemon

```bash
shadow stop
```

### Full uninstall

```bash
# Remove daemon, keep managed blocks in agent files
shadow uninstall

# Remove daemon AND clean managed blocks from all agent files
shadow uninstall --clean-blocks
```

Your data stays at `~/.shadow/` after uninstall.
Delete it manually for a clean slate:

```bash
rm -rf ~/.shadow
```

## Troubleshooting

### "Shadow daemon is not running"

```bash
shadow start    # Start it
shadow status   # Verify
```

### "Cannot reach Shadow daemon"

The daemon may still be starting. Wait a few seconds and retry:

```bash
shadow stop && shadow start
```

### Port 7878 is already in use

```bash
lsof -i :7878         # Check what's using the port
shadow stop            # Stop Shadow if needed
shadow start           # Restart
```

### No candidate rules appearing

- Make sure you're in a Git repository
- Verify capture is enabled in **Settings** (web console)
- Check that your agent is writing session logs
  (Claude Code writes to `~/.claude/projects/`)
- Check daemon logs: `~/.shadow/logs/`

### Managed blocks not appearing in agent files

- Verify the adapter is enabled in **Settings**
- Trigger a manual sync:

```bash
curl -X POST http://localhost:7878/api/adapters/sync
```

### Build from source fails

- Go 1.25+: `go version`
- Node.js 22+: `node --version`
- Run `make web-setup` before `make build`
- Web assets must build before the Go binary (embedded)

---

**Questions? Issues?** [Open an issue](https://github.com/joevilcai666/shadow/issues) — we're happy to help.
