# Distribution Strategy

## Principle

Meet developers where they already are. Every developer should be able to install csaw with a command they already know.

## Channels

### Phase 1 — Launch

Handled by GoReleaser from a single `.goreleaser.yml`, triggered by a git tag.

| Channel | Command | Audience |
|---|---|---|
| GitHub Releases | Download binary | Everyone (baseline) |
| Homebrew Cask | `brew install --cask NicholasCullenCooper/tap/csaw` | macOS and Linux devs |
| Scoop | `scoop install csaw` | Windows devs |
| PyPI | `uv tool install csaw` | Python ecosystem / cross-platform |

**Repos:**
- `NicholasCullenCooper/homebrew-tap` — GoReleaser auto-populates the cask on release
- `NicholasCullenCooper/scoop-bucket` — GoReleaser auto-populates the manifest on release

**PyPI** — uses [go-to-wheel](https://github.com/simonw/go-to-wheel) to package compiled Go binaries as Python wheels. Each wheel embeds the binary for a specific platform (e.g., `macosx_11_0_arm64`, `manylinux_2_17_x86_64`). pip/uv automatically selects the correct wheel. A thin Python wrapper calls `os.execvp()` to run the embedded binary. Build script: `scripts/build-pypi-wheels.py`. Published via trusted publishing (OIDC) in the `pypi` job of the release workflow.

### Phase 2 — Growth

| Channel | Command | Audience |
|---|---|---|
| npm | `npx csaw` / `npm install -g csaw` | Node/frontend ecosystem |

**npm** — follow the [esbuild pattern](https://github.com/evanw/esbuild/issues/789): publish scoped platform packages (`@NicholasCullenCooper/csaw-darwin-arm64`, `@NicholasCullenCooper/csaw-linux-x64`, etc.) as optional dependencies of a main `csaw` package. npm installs only the matching platform package. A thin JS wrapper finds and execs the binary. No postinstall scripts.

### Phase 3 — Completeness

| Channel | Command | Audience |
|---|---|---|
| Winget | `winget install NicholasCullenCooper.csaw` | Windows corporate |
| Nix | `nix run github:NicholasCullenCooper/csaw` | NixOS users |
| Docker | `docker run ghcr.io/NicholasCullenCooper/csaw` | CI pipelines |
| AUR | `yay -S csaw` | Arch Linux |

GoReleaser supports Winget manifest generation, Nix package generation, Docker images, and AUR out of the box.

## Release workflow

One GitHub Actions workflow triggered by pushing a version tag.

### How to cut a release

1. **Ensure main is clean and CI has passed.** All commits should already be on `origin/main`.

2. **Choose the version.** Follow semver:
   - **Patch** (`v0.1.1`): bug fixes, small additions that don't change behavior
   - **Minor** (`v0.2.0`): new features, new commands, new config surface
   - **Major** (`v1.0.0`): breaking changes to CLI, config format, or mount behavior

3. **Tag and push:**
   ```bash
   git tag -a v0.1.1 -m "v0.1.1: short description of what changed"
   git push origin v0.1.1
   ```

4. **Verify the release.** Check the GitHub Actions run and the resulting GitHub Release page.

The tag push triggers the full pipeline automatically:

```
git tag -a v0.1.1 -m "..." && git push origin v0.1.1

  GoReleaser job:
    ├── Cross-compile (linux/mac/win x amd64/arm64)
    ├── GitHub Release + checksums + changelog
    ├── Homebrew cask → NicholasCullenCooper/homebrew-tap
    └── Scoop manifest → NicholasCullenCooper/scoop-bucket

  PyPI job (runs after GoReleaser):
    ├── go-to-wheel builds 8 platform wheels
    └── Trusted publishing (OIDC) → pypi.org/project/csaw
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
brew install --cask NicholasCullenCooper/tap/csaw

# Windows
scoop bucket add NicholasCullenCooper https://github.com/NicholasCullenCooper/scoop-bucket
scoop install csaw

# Any platform (recommended)
uv tool install csaw
csaw mount --profile backend

# Node ecosystem (Phase 2)
npx csaw mount --profile backend

# Verify
csaw version
```

The binary is identical across all channels. Only the packaging differs.

## Infrastructure checklist

| What | Effort | Needed by |
|---|---|---|
| `.goreleaser.yml` in this repo | Low | First release |
| `NicholasCullenCooper/homebrew-tap` GitHub repo | Trivial | First release |
| `NicholasCullenCooper/scoop-bucket` GitHub repo | Trivial | First release |
| GitHub Actions release workflow | Medium | First release |
| ~~PyPI publishing workflow (go-to-wheel)~~ | ~~Medium~~ | ~~Done (v0.1.2)~~ |
| npm packages (esbuild pattern) | Medium | Phase 2 |
| Winget / Nix / Docker / AUR | Low each | Phase 3 |
