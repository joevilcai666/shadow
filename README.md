# Shadow

> Your AI agent memory layer — correct once, remember everywhere.

Shadow is a local-first daemon that captures your corrections to coding agents (Claude Code, Cursor, Codex, etc.) and turns them into persistent rules that work across all your tools.

## Quick Start

```bash
# Build from source
make build

# Run
./shadow version
./shadow start   # Onboarding wizard (coming soon)
./shadow serve   # Start daemon + web console (coming soon)
```

## Development

```bash
# Install web dependencies
make web-setup

# Start dev environment (backend + web hot-reload)
make dev

# Run tests
make test

# Lint
make lint
```

## Architecture

```
Shadow/
├── cmd/shadow/          — CLI entry point
├── internal/
│   ├── daemon/          — Daemon core (process management, state machine)
│   ├── capture/         — Capture engine (log reading, signal extraction)
│   ├── distill/         — Rule distillation engine ("translator")
│   ├── adapter/         — Adapter layer (Claude Code / Cursor / Codex)
│   ├── storage/         — Storage layer (SQLite CRUD)
│   ├── config/          — Configuration management
│   └── server/          — HTTP API + WebSocket + embedded web UI
├── web/                 — React SPA (Vite + TypeScript + Tailwind)
├── migrations/          — SQLite migration files
├── Makefile
└── go.mod
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Daemon / CLI | Go (cobra + bubbletea) |
| Storage | SQLite (modernc.org/sqlite — pure Go, no CGO) |
| Web UI | React + Vite + TypeScript + Tailwind CSS |
| Embed | Go embed for single-binary distribution |

## License

MIT
