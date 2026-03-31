# Distribution Strategy

## Principle

Meet developers where they already are. Every developer should be able to install csaw with a command they already know.

## Channels

### Phase 1 — Launch

Handled by GoReleaser from a single `.goreleaser.yml`, triggered by a git tag.

| Channel | Command | Audience |
|---|---|---|
| GitHub Releases | Download binary | Everyone (baseline) |
| Homebrew Cask | `brew install --cask csaw-ai/tap/csaw` | macOS and Linux devs |
| Scoop | `scoop install csaw` | Windows devs |

**Repos to create:**
- `csaw-ai/homebrew-tap` — GoReleaser auto-populates the cask on release
- `csaw-ai/scoop-bucket` — GoReleaser auto-populates the manifest on release

### Phase 2 — Growth

Add PyPI and npm to reach developers through package managers they already use daily.

| Channel | Command | Audience |
|---|---|---|
| PyPI | `pip install csaw` / `uvx csaw` | Python ecosystem |
| npm | `npx csaw` / `npm install -g csaw` | Node/frontend ecosystem |

**PyPI** — use [go-to-wheel](https://github.com/simonw/go-to-wheel) to package compiled Go binaries as Python wheels. Each wheel embeds the binary for a specific platform (e.g., `macosx_11_0_arm64`, `manylinux_2_17_x86_64`). pip/uv automatically selects the correct wheel for the user's OS and architecture. A thin Python wrapper (`csaw/__init__.py`) calls `os.execvp()` to run the embedded binary. The `csaw` name is already reserved on PyPI.

**npm** — follow the [esbuild pattern](https://github.com/evanw/esbuild/issues/789): publish scoped platform packages (`@csaw-ai/csaw-darwin-arm64`, `@csaw-ai/csaw-linux-x64`, etc.) as optional dependencies of a main `csaw` package. npm installs only the matching platform package. A thin JS wrapper finds and execs the binary. No postinstall scripts.

### Phase 3 — Completeness

| Channel | Command | Audience |
|---|---|---|
| Winget | `winget install csaw-ai.csaw` | Windows corporate |
| Nix | `nix run github:csaw-ai/csaw` | NixOS users |
| Docker | `docker run ghcr.io/csaw-ai/csaw` | CI pipelines |
| AUR | `yay -S csaw` | Arch Linux |

GoReleaser supports Winget manifest generation, Nix package generation, Docker images, and AUR out of the box.

## Release workflow

One GitHub Actions workflow triggered by pushing a version tag:

```
git tag v0.1.0 && git push --tags

  GoReleaser job:
    ├── Cross-compile (linux/mac/win x amd64/arm64)
    ├── GitHub Release + checksums + changelog
    ├── Homebrew cask → csaw-ai/homebrew-tap
    └── Scoop manifest → csaw-ai/scoop-bucket

  PyPI job (parallel, Phase 2):
    ├── go-to-wheel builds platform wheels
    └── twine upload dist/*

  npm job (parallel, Phase 2):
    ├── Copy binaries into @csaw-ai/csaw-{platform} packages
    └── npm publish for each
```

## Version injection

GoReleaser sets the version at build time via ldflags:

```yaml
# .goreleaser.yml
builds:
  - ldflags:
      - -s -w -X main.version={{.Version}}
```

This populates the `version` variable in `cmd/csaw/root.go` that `csaw version` prints.

## What the user sees

```bash
# macOS / Linux
brew install --cask csaw-ai/tap/csaw

# Windows
scoop bucket add csaw-ai https://github.com/csaw-ai/scoop-bucket
scoop install csaw

# Python ecosystem
pip install csaw
uvx csaw mount --profile backend

# Node ecosystem
npx csaw mount --profile backend

# Verify
csaw version
```

The binary is identical across all channels. Only the packaging differs.

## Infrastructure checklist

| What | Effort | Needed by |
|---|---|---|
| `.goreleaser.yml` in this repo | Low | First release |
| `csaw-ai/homebrew-tap` GitHub repo | Trivial | First release |
| `csaw-ai/scoop-bucket` GitHub repo | Trivial | First release |
| GitHub Actions release workflow | Medium | First release |
| PyPI publishing workflow (go-to-wheel) | Medium | Phase 2 |
| npm packages (esbuild pattern) | Medium | Phase 2 |
| Winget / Nix / Docker / AUR | Low each | Phase 3 |
