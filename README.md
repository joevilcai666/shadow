# Shadow

> Your AI agent memory layer: correct once, remember everywhere.

Shadow is a local-first macOS daemon for heavy coding-agent users. It captures correction signals, turns them into reviewable rules, and writes approved rules into the native context files your agents already read: `CLAUDE.md`, `.cursorrules`, `AGENTS.md`, and `.github/copilot-instructions.md`.

## Install

```bash
brew tap joevilcai666/shadow
brew install --formula joevilcai666/shadow/shadow
```

Or download a macOS archive from [GitHub Releases](https://github.com/joevilcai666/shadow/releases).

## First Run

```bash
shadow start
```

`shadow start` registers the daemon with launchd, opens the terminal onboarding flow, scans the current project for initial candidate memories, installs safe git hooks when the current directory is a Git repo, and then offers to open the local web console.

Approved rules sync into agent context files. New rules stay `candidate` until you approve them.

## Commands

```bash
shadow status      # Check daemon status
shadow open        # Open http://localhost:7878
shadow review      # Review candidate rules in the terminal
shadow serve       # Run the daemon in the foreground
shadow stop        # Stop the launchd daemon
shadow mcp         # Print MCP HTTP wiring
shadow uninstall --clean-blocks
```

## Local-First Promise

- Data stays under `~/.shadow/` by default.
- No login is required for the local product chain.
- Raw secrets are blocked by deny patterns before storage.
- Managed blocks are removable with `shadow uninstall --clean-blocks`.

## Development

```bash
make web-setup
make build
make test
make vet
```

Useful targets:

```bash
make dev         # Go daemon + Vite dev server
make web-static  # Build web/dist and copy it into internal/server/static
```

## Release Smoke Test

```bash
make build
./shadow version
./shadow serve
./shadow open
```

For release packaging:

```bash
goreleaser release --snapshot --clean
```

## License

MIT
