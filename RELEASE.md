# Release Guide

## Automated Release (via GoReleaser)

```bash
# Tag and push to trigger the release workflow
make release v=0.2.0
```

This will:
1. Create a git tag `v0.2.0`
2. Push it to GitHub
3. Trigger the Release workflow (`.github/workflows/release.yml`)
4. GoReleaser builds for darwin (arm64/amd64) + linux
5. Creates a GitHub Release with checksums
6. Pushes the Homebrew formula to `joevilcai666/homebrew-shadow`

## Prerequisites

### 1. Create the Homebrew tap repo

```bash
gh repo create joevilcai666/homebrew-shadow --public
```

### 2. Set up GitHub Secrets

In the `joevilcai666/shadow` repo settings → Secrets:

| Secret | Description |
|--------|-------------|
| `HOMEBREW_TAP_TOKEN` | A GitHub PAT with `repo` scope, for pushing to `homebrew-shadow` |

### 3. Install GoReleaser (for local testing)

```bash
brew install goreleaser
```

## Local Release Test (dry-run)

```bash
goreleaser release --snapshot --clean
```

## Manual Homebrew Install (after release)

Users install via:
```bash
brew tap joevilcai666/shadow
brew install shadow
```

## Manual Install (without Homebrew)

```bash
# Download from GitHub Releases
curl -sL https://github.com/joevilcai666/shadow/releases/latest/download/shadow_0.2.0_darwin_arm64.tar.gz | tar xz
sudo mv shadow /usr/local/bin/
```
