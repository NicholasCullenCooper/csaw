package drift

import (
	"os"
	"path/filepath"

	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

const (
	IssueMissingSource           = "missing-source"
	IssueDriftedLink             = "drifted-link"
	IssueMissingLink             = "missing-link"
	IssueReplacedLink            = "replaced-link"
	IssueProtectedContentDrift   = "protected-content-drift"
	IssueProtectedHashUnreadable = "protected-hash-unreadable"
	IssueGeneratedModified       = "generated-modified"
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
	// Generated marks a file csaw wrote from content rather than linked.
	Generated bool
}

// inspectGenerated classifies a generated file: missing, replaced by a
// non-regular file, modified by the user, or healthy.
func inspectGenerated(status Status, fullPath string, expectedHash string) Status {
	status.ExpectedHash = expectedHash
	info, err := os.Lstat(fullPath)
	if err != nil {
		status.Healthy = false
		if os.IsNotExist(err) {
			status.Issue = IssueMissingLink
		} else {
			status.Issue = IssueReplacedLink
		}
		return status
	}
	if !info.Mode().IsRegular() {
		status.Healthy = false
		status.Issue = IssueReplacedLink
		return status
	}
	actualHash, err := workspace.FileSHA256(fullPath)
	if err != nil {
		status.Healthy = false
		status.Issue = IssueProtectedHashUnreadable
		return status
	}
	status.ActualHash = actualHash
	if actualHash != expectedHash {
		status.Healthy = false
		status.Issue = IssueGeneratedModified
	}
	return status
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
		}

		if entry.Generated {
			statuses = append(statuses, inspectGenerated(status, fullPath, entry.ContentSHA256))
			continue
		}

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
			statuses = append(statuses, status)
			continue
		}

		if !linkmode.IsLink(lm, fullPath, entry.SourcePath) {
			status.Healthy = false
			status.Issue = IssueReplacedLink
			statuses = append(statuses, status)
			continue
		}

		healthy, resolved := linkmode.Verify(lm, fullPath, entry.SourcePath, runtime.PathsEqual)
		status.ResolvedTarget = runtime.NormalizeFSPath(resolved)
		status.ActualTarget = resolved

		if status.Issue == IssueMissingSource {
			statuses = append(statuses, status)
			continue
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

		statuses = append(statuses, status)
	}

	return statuses
}
