# P0 Release Guide

Shadow P0 ships through GitHub Releases and Homebrew for macOS users.

## Prerequisites

```bash
brew install goreleaser
gh repo create joevilcai666/homebrew-shadow --public
```

Set `HOMEBREW_TAP_TOKEN` in `joevilcai666/shadow` repository secrets. The token needs permission to push to `joevilcai666/homebrew-shadow`.

## Local Checklist

```bash
make web-setup
make web-static
make test
make vet
npm --prefix web run lint
make build
./shadow version
```

Run the local product smoke path from a temporary Git repo:

```bash
tmpdir="$(mktemp -d)"
cd "$tmpdir"
git init
go mod init example.com/shadow-smoke
printf "Always write table-driven tests.\\n" > CLAUDE.md
shadow serve
```

In another terminal:

```bash
shadow open
curl -s http://localhost:7878/api/rules?status=candidate
curl -s -X POST http://localhost:7878/api/adapters/sync
```

Approve a candidate in the web console or with:

```bash
shadow review
```

Verify a Shadow managed block appears in enabled project context files.

## Snapshot Build

```bash
goreleaser release --snapshot --clean
dist/shadow_darwin_arm64*/shadow version
```

Install the built artifact manually:

```bash
sudo cp dist/shadow_darwin_arm64*/shadow /usr/local/bin/shadow
shadow version
shadow serve
shadow open
```

## Tagged Release

```bash
make release v=0.2.0
```

The GitHub workflow will:

1. Check out the tag.
2. Install Go and Node.
3. Run GoReleaser.
4. Build fresh web static assets through `make web-static`.
5. Cross-build pure-Go binaries with `CGO_ENABLED=0`.
6. Publish GitHub Release archives and checksums.
7. Push the Homebrew formula to `joevilcai666/homebrew-shadow`.

## Homebrew Verification

After the release finishes:

```bash
brew update
brew tap joevilcai666/shadow
brew install shadow
shadow version
shadow start
shadow status
shadow open
```

## P0 Gates

- New onboarding-generated project rules include `project_path`.
- Candidate rules remain inactive until explicit approval.
- Rule activation, edit, delete, batch update, rollback, and adapter toggles trigger adapter sync.
- Disabling an adapter removes Shadow managed blocks for that adapter.
- Web console builds with HeroUI and route-level chunks.
- `go test` and `go vet` exclude Go packages inside `web/node_modules`.
- Snapshot release embeds freshly built web static assets.
