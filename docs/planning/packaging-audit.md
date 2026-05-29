# Packaging Audit

Landscape map of cross-platform distribution channels for csaw, and a decision per channel about whether to add it. Captured 2026-05-26.

**Stance: nothing is added in this pass.** The audit exists so future-us doesn't re-derive the landscape, and so that when a real user reports friction (`"I'm on Fedora and tar.gz install is annoying"`), we already know exactly what to add and how. Adding channels speculatively is feature parity, not user-driven product work.

## Current state (v0.8.2)

From `.goreleaser.yml` + `.github/workflows/release.yml`:

| Channel | Status | Mechanism |
|---|---|---|
| GitHub releases (tar.gz, zip) | ✅ Active | GoReleaser `archives:` block |
| Homebrew cask | ✅ Active | GoReleaser `homebrew_casks:` → `homebrew-tap` repo |
| Scoop | ✅ Active | GoReleaser `scoops:` → `scoop-bucket` repo |
| PyPI | ✅ Active | Separate `pypi` job using `uv` after `release` completes |

**PyPI is the cross-platform Linux story today.** `uv tool install csaw` works on any Linux distro. Native package managers (apt/dnf/apk) would be additive, not necessary.

## Decision per channel

### Add when a user asks

These are all cheap (~5 lines of GoReleaser config) and well-supported. The trigger is a real user reporting friction, not "cc-switch has it."

- **nfpms (DEB + RPM + APK)** — `nfpms:` block, attaches packages to GitHub releases. Linux users install with `sudo dpkg -i csaw_*.deb` / `sudo rpm -i csaw_*.rpm` / `sudo apk add --allow-untrusted csaw_*.apk`. PyPI already covers the cross-distro install case; native packages are nice-to-have.
- **winget** — Microsoft Package Manager (`winget:` block, PRs to `microsoft/winget-pkgs`). Growing default Windows install path; complements Scoop. Add when a non-Scoop Windows user reports friction.
- **AUR** — `aurs:` block, AUR account + SSH key. One-time setup. Add when an Arch-using csaw user reports the friction.

### Add only with significant demand

These need more setup (accounts, signing, ongoing review processes).

- **Snap** — Ubuntu store credentials + store review process. Add only if multiple Snap-only users emerge.
- **Chocolatey** — Windows alternative to Scoop with chocolatey.org account + signing. Most dev-tool Windows users use Scoop; Chocolatey serves a different segment.
- **Hosted apt/yum repos** (Cloudsmith, JFrog, Fury.io) — Real ongoing cost (signing keys, repo metadata, hosting). Defer until "local install from release asset" friction is widely reported.

### Defer to community

- **Flatpak** — Typically community-maintained for CLI tools. Not a GoReleaser channel; would need a separate Flathub manifest. Defer indefinitely; if someone builds one in the community, link from README.

### Never

- **Docker images** — csaw operates on the user's local filesystem and git checkouts. A container won't have access to the user's source repos, AI tool dirs, or git state. Wrong shape.
- **Homebrew formula (vs. cask)** — Cask is for pre-built binaries (csaw's case). Formula is for compile-from-source. Cask is correct.

## How to act on this doc

When a user reports install friction:

1. Identify their platform + package manager preference.
2. Match against the table above.
3. If in "Add when a user asks": one PR adds the GoReleaser block; next tag publishes.
4. If in "Add only with significant demand": collect 2-3 user reports before swinging.
5. If in "Defer to community" / "Never": respond with the rationale, link this doc.

Do NOT add a channel because a competing tool ships it. Add because a real user can't install csaw easily today.
