package drift

import (
	"os"
	"path/filepath"

	"github.com/NicholasCullenCooper/csaw/internal/hooks"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

// Issue names are part of the audit JSON contract (see
// docs/reference/audit-json.md). Link issues apply to symlink-mode entries;
// file issues apply to copy-mode and generated entries.
const (
	IssueMissingSource           = "missing-source"
	IssueDriftedLink             = "drifted-link"
	IssueMissingLink             = "missing-link"
	IssueReplacedLink            = "replaced-link"
	IssueProtectedContentDrift   = "protected-content-drift"
	IssueProtectedHashUnreadable = "protected-hash-unreadable"

	// IssueMissingFile: a copied or generated file is gone from the project.
	IssueMissingFile = "missing-file"
	// IssueReplacedFile: the project path is no longer a regular file.
	IssueReplacedFile = "replaced-file"
	// IssueModifiedLocally: the project copy no longer matches the bytes csaw
	// wrote. csaw never overwrites it; unmount preserves it.
	IssueModifiedLocally = "modified-locally"
	// IssueStale: the source changed since the copy was made and the project
	// copy is untouched. `csaw update` refreshes it.
	IssueStale = "stale"
	// IssueStaleAndModified: both the source and the project copy changed.
	IssueStaleAndModified = "stale-and-modified"
	// IssueUnreadable: the project file could not be hashed.
	IssueUnreadable = "unreadable"
	// IssueHooksMissing: some hook groups csaw merged into a tool's hooks
	// file are no longer there. `csaw update` merges them again.
	IssueHooksMissing = "hooks-missing"
)

type Status struct {
	Healthy        bool
	RelativePath   string
	FullPath       string
	SourceName     string
	ExpectedSource string
	ActualTarget   string
	ResolvedTarget string
	Issue          string
	Protected      bool
	ExpectedHash   string
	ActualHash     string
	// SourceHash is the current hash of the source file (copy mode).
	SourceHash string
	// Generated marks a file csaw wrote from content rather than linked.
	Generated bool
	// Copy marks a copy-mode entry (a plain file tracked by hash).
	Copy bool
}

// NeedsUpdate reports whether `csaw update` can bring the entry back to
// health without discarding anything: missing or stale copies, missing or
// drifted links.
func (s Status) NeedsUpdate() bool {
	switch s.Issue {
	case IssueMissingFile, IssueStale, IssueMissingLink, IssueDriftedLink, IssueHooksMissing:
		return true
	}
	return false
}

// LocallyModified reports whether the project file carries edits csaw must
// not overwrite.
func (s Status) LocallyModified() bool {
	switch s.Issue {
	case IssueModifiedLocally, IssueStaleAndModified:
		return true
	case IssueProtectedContentDrift:
		return s.Copy || s.Generated
	}
	return false
}

func InspectLinks(links []workspace.MountedLink) []Status {
	statuses := make([]Status, 0, len(links))
	for _, link := range links {
		status := Status{
			Healthy:        true,
			RelativePath:   link.RelativePath,
			FullPath:       link.FullPath,
			ResolvedTarget: link.ResolvedTarget,
		}

		if _, err := os.Stat(link.ResolvedTarget); err != nil {
			status.Healthy = false
			status.Issue = IssueMissingSource
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func InspectMountState(projectRoot string, state workspace.MountState, lm linkmode.Mode) []Status {
	statuses := make([]Status, 0, len(state.Entries))
	for _, entry := range state.Entries {
		fullPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
		status := Status{
			Healthy:        true,
			RelativePath:   entry.RelativePath,
			FullPath:       fullPath,
			SourceName:     entry.SourceName,
			ExpectedSource: entry.SourcePath,
			Protected:      entry.Protected,
			ExpectedHash:   entry.SourceSHA256,
			Generated:      entry.Generated,
			Copy:           entry.Mode == "copy",
		}

		switch {
		case entry.IsMerge():
			statuses = append(statuses, inspectMergedHooks(status, fullPath, entry))
		case entry.Generated:
			statuses = append(statuses, inspectGenerated(status, fullPath, entry.ContentSHA256))
		case entry.Mode == "copy":
			statuses = append(statuses, inspectCopy(status, fullPath, entry))
		default:
			statuses = append(statuses, inspectLink(status, fullPath, entry, lm))
		}
	}

	return statuses
}

// inspectMergedHooks checks that every hook group csaw merged into a tool's
// hooks file is still present.
func inspectMergedHooks(status Status, fullPath string, entry workspace.MountedStateEntry) Status {
	status.ExpectedHash = entry.ContentSHA256
	owned, err := hooks.Decode(entry.Content)
	if err != nil {
		status.Healthy = false
		status.Issue = IssueUnreadable
		return status
	}
	if _, err := os.Stat(fullPath); err != nil {
		status.Healthy = false
		status.Issue = IssueMissingFile
		return status
	}
	_, missing, err := hooks.Inspect(fullPath, owned)
	if err != nil {
		status.Healthy = false
		status.Issue = IssueUnreadable
		return status
	}
	if missing > 0 {
		status.Healthy = false
		status.Issue = IssueHooksMissing
	}
	return status
}

// inspectGenerated classifies a generated file: missing, replaced by a
// non-regular file, modified by the user, or healthy.
func inspectGenerated(status Status, fullPath string, expectedHash string) Status {
	status.ExpectedHash = expectedHash
	info, err := os.Lstat(fullPath)
	if err != nil {
		status.Healthy = false
		if os.IsNotExist(err) {
			status.Issue = IssueMissingFile
		} else {
			status.Issue = IssueReplacedFile
		}
		return status
	}
	if !info.Mode().IsRegular() {
		status.Healthy = false
		status.Issue = IssueReplacedFile
		return status
	}
	actualHash, err := workspace.FileSHA256(fullPath)
	if err != nil {
		status.Healthy = false
		status.Issue = IssueUnreadable
		return status
	}
	status.ActualHash = actualHash
	if actualHash != expectedHash {
		status.Healthy = false
		status.Issue = IssueModifiedLocally
	}
	return status
}

// inspectCopy classifies a copy-mode entry by comparing three hashes: the
// bytes csaw wrote (recorded), the project file now, and the source now.
func inspectCopy(status Status, fullPath string, entry workspace.MountedStateEntry) Status {
	recorded := entry.ContentSHA256
	status.ExpectedHash = recorded

	sourceHash, err := workspace.FileSHA256(entry.SourcePath)
	if err != nil {
		status.Healthy = false
		status.Issue = IssueMissingSource
		return status
	}
	status.SourceHash = sourceHash

	info, err := os.Lstat(fullPath)
	if err != nil {
		status.Healthy = false
		if os.IsNotExist(err) {
			status.Issue = IssueMissingFile
		} else {
			status.Issue = IssueReplacedFile
		}
		return status
	}
	if !info.Mode().IsRegular() {
		status.Healthy = false
		status.Issue = IssueReplacedFile
		return status
	}

	projectHash, err := workspace.FileSHA256(fullPath)
	if err != nil {
		status.Healthy = false
		status.Issue = IssueUnreadable
		return status
	}
	status.ActualHash = projectHash

	modified := projectHash != recorded
	stale := sourceHash != recorded
	switch {
	case entry.Protected && modified:
		status.Healthy = false
		status.Issue = IssueProtectedContentDrift
	case stale && modified:
		status.Healthy = false
		status.Issue = IssueStaleAndModified
	case modified:
		status.Healthy = false
		status.Issue = IssueModifiedLocally
	case stale:
		status.Healthy = false
		status.Issue = IssueStale
	}
	return status
}

// inspectLink classifies a symlink-mode entry (the pre-v0.12 behavior).
func inspectLink(status Status, fullPath string, entry workspace.MountedStateEntry, lm linkmode.Mode) Status {
	if _, err := os.Stat(entry.SourcePath); err != nil {
		status.Healthy = false
		status.Issue = IssueMissingSource
	}

	_, err := os.Lstat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			status.Healthy = false
			status.Issue = IssueMissingLink
		}
		return status
	}

	if !linkmode.IsLink(lm, fullPath, entry.SourcePath) {
		status.Healthy = false
		status.Issue = IssueReplacedLink
		return status
	}

	healthy, resolved := linkmode.Verify(lm, fullPath, entry.SourcePath, runtime.PathsEqual)
	status.ResolvedTarget = runtime.NormalizeFSPath(resolved)
	status.ActualTarget = resolved

	if status.Issue == IssueMissingSource {
		return status
	}

	if !healthy {
		status.Healthy = false
		status.Issue = IssueDriftedLink
	}

	if status.Healthy && status.Protected && status.ExpectedHash != "" {
		actualHash, err := workspace.FileSHA256(fullPath)
		if err != nil {
			status.Healthy = false
			status.Issue = IssueProtectedHashUnreadable
		} else {
			status.ActualHash = actualHash
			if actualHash != status.ExpectedHash {
				status.Healthy = false
				status.Issue = IssueProtectedContentDrift
			}
		}
	}

	return status
}
