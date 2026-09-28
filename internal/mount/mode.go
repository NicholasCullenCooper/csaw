package mount

import "fmt"

// MountMode selects how csaw places a registry file in a project.
//
// Copy writes the bytes into the project and tracks them by SHA-256, so the
// file is a plain file to every tool (Claude Code refuses rule symlinks that
// point outside the working directory), no symlink privilege is needed, and
// a project lockfile can pin exact content. csaw removes only bytes it
// wrote; edits are preserved under .csaw-stash/modified/ on unmount.
//
// Symlink links the project path to the source checkout, so edits flow both
// ways instantly. It is the pre-v0.12 behavior and stays available as an
// opt-in.
type MountMode string

const (
	ModeCopy    MountMode = "copy"
	ModeSymlink MountMode = "symlink"
)

// DefaultMountMode is used when neither config nor a flag names a mode.
const DefaultMountMode = ModeCopy

// ParseMountMode maps user input to a MountMode. Empty input means the default.
func ParseMountMode(value string) (MountMode, error) {
	switch MountMode(value) {
	case "":
		return DefaultMountMode, nil
	case ModeCopy, ModeSymlink:
		return MountMode(value), nil
	default:
		return "", fmt.Errorf("unknown mount mode %q (valid: copy, symlink)", value)
	}
}

// ApplyOptions controls how Apply places files.
type ApplyOptions struct {
	Mode MountMode
}
