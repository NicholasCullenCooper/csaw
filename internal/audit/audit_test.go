package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/pinning"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

func TestLoadPolicyParsesGovernanceChecks(t *testing.T) {
	project := t.TempDir()
	writePolicy(t, project, `
required_sources:
  - team
  - name: client
    url: git@example.com:org/client-ai.git
    ref: main
blocked_sources:
  - personal
  - other-client-*
blocked_kinds:
  - mcp
blocked_paths:
  - .claude/agents/**
required_kinds:
  - instructions
  - agents
`)

	policy, policyPath, found, err := LoadPolicy(project)
	if err != nil {
		t.Fatalf("LoadPolicy() error = %v", err)
	}
	if !found {
		t.Fatal("LoadPolicy() found = false, want true")
	}
	if filepath.Base(policyPath) != PolicyFileName {
		t.Fatalf("policy path = %q, want %s", policyPath, PolicyFileName)
	}
	if got, want := len(policy.RequiredSources), 2; got != want {
		t.Fatalf("required sources = %d, want %d", got, want)
	}
	if policy.RequiredSources[1].Name != "client" || policy.RequiredSources[1].Ref != "main" {
		t.Fatalf("object source requirement not parsed: %+v", policy.RequiredSources[1])
	}
	if got, want := policy.BlockedSources[1], "other-client-*"; got != want {
		t.Fatalf("blocked source = %q, want %q", got, want)
	}
	if got, want := policy.BlockedKinds[0], mount.KindMCP; got != want {
		t.Fatalf("blocked kind = %q, want %q", got, want)
	}
	if got, want := policy.BlockedPaths[0], ".claude/agents/**"; got != want {
		t.Fatalf("blocked path = %q, want %q", got, want)
	}
	if got, want := policy.RequiredKinds[1], mount.KindAgent; got != want {
		t.Fatalf("required kind = %q, want %q", got, want)
	}
}

func TestRunPassesWhenRequiredContextIsActive(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	writePolicy(t, project, `
required_sources:
  - team
blocked_sources:
  - personal
blocked_kinds:
  - mcp
blocked_paths:
  - .claude/agents/**
required_kinds:
  - instructions
`)
	writeMountedFile(t, project, "AGENTS.md", "team", "instructions")

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Failed(false) {
		t.Fatalf("report should pass, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "source.required.present", SeverityOK)
	assertFinding(t, report, "source.blocked.clear", SeverityOK)
	assertFinding(t, report, "kind.blocked.clear", SeverityOK)
	assertFinding(t, report, "path.blocked.clear", SeverityOK)
	assertFinding(t, report, "kind.required.present", SeverityOK)
}

func TestRunChecksRequiredSourceURLAndRef(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	writeSourceConfig(t, paths, sources.Source{
		Name: "client",
		Kind: sources.KindRemote,
		URL:  "git@example.com:org/client-ai.git",
	})
	writePinState(t, project, pinning.Pin{Source: "client", Ref: "main"})
	writePolicy(t, project, `
required_sources:
  - name: client
    url: git@example.com:org/client-ai.git
    ref: main
`)
	writeMountedFile(t, project, "AGENTS.md", "client", "instructions")

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Failed(false) {
		t.Fatalf("report should pass, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "source.required.present", SeverityOK)
	assertFinding(t, report, "source.required.url_match", SeverityOK)
	assertFinding(t, report, "source.required.ref_match", SeverityOK)
}

func TestRunFailsForRequiredSourceURLAndRefMismatch(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	writeSourceConfig(t, paths, sources.Source{
		Name: "client",
		Kind: sources.KindRemote,
		URL:  "git@example.com:org/other-ai.git",
	})
	writePolicy(t, project, `
required_sources:
  - name: client
    url: git@example.com:org/client-ai.git
    ref: main
`)
	writeMountedFile(t, project, "AGENTS.md", "client", "instructions")

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Failed(false) {
		t.Fatalf("report should fail, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "source.required.present", SeverityOK)
	assertFinding(t, report, "source.required.url_mismatch", SeverityError)
	assertFinding(t, report, "source.required.ref_mismatch", SeverityError)
}

func TestRunFailsForMissingAndBlockedContext(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	writePolicy(t, project, `
required_sources:
  - team
blocked_sources:
  - personal
required_kinds:
  - mcp
`)
	writeMountedFile(t, project, "AGENTS.md", "personal", "instructions")

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Failed(false) {
		t.Fatalf("report should fail, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "source.required.missing", SeverityError)
	assertFinding(t, report, "source.blocked.active", SeverityError)
	assertFinding(t, report, "kind.required.missing", SeverityError)
}

func TestRunFailsForBlockedKind(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	writePolicy(t, project, `
blocked_kinds:
  - mcp
`)
	writeMountedFile(t, project, ".mcp.json", "personal", "{}")

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Failed(false) {
		t.Fatalf("report should fail, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "kind.blocked.active", SeverityError)
}

func TestRunFailsForBlockedPath(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	writePolicy(t, project, `
blocked_paths:
  - .claude/agents/**
`)
	writeMountedFile(t, project, ".claude/agents/reviewer.md", "personal", "agent")

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Failed(false) {
		t.Fatalf("report should fail, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "path.blocked.active", SeverityError)
	assertFindingDetail(t, report, "path.blocked.active", "matched .claude/agents/**")
}

func TestRunFailsForProtectedContentDrift(t *testing.T) {
	project := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(t.TempDir(), ".csaw"))
	source := filepath.Join(t.TempDir(), "AGENTS.md")
	target := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(source, []byte("approved"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	approvedHash, err := workspace.FileSHA256(source)
	if err != nil {
		t.Fatalf("FileSHA256() error = %v", err)
	}
	if err := linkmode.Create(linkmode.Detect(), source, target); err != nil {
		t.Fatalf("Create link error = %v", err)
	}
	if err := os.WriteFile(source, []byte("changed"), 0o644); err != nil {
		t.Fatalf("WriteFile(changed source) error = %v", err)
	}
	state := workspace.MountState{
		Entries: []workspace.MountedStateEntry{{
			RelativePath: "AGENTS.md",
			SourceName:   "team",
			SourcePath:   source,
			Protected:    true,
			SourceSHA256: approvedHash,
		}},
	}
	if err := workspace.WriteMountState(project, state); err != nil {
		t.Fatalf("WriteMountState() error = %v", err)
	}

	report, err := Run(project, paths)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Failed(false) {
		t.Fatalf("report should fail, findings: %+v", report.Findings)
	}
	assertFinding(t, report, "mount.unhealthy", SeverityError)
	assertFindingDetail(t, report, "mount.unhealthy", drift.IssueProtectedContentDrift)
}

func TestStrictModeFailsWarnings(t *testing.T) {
	report := Report{
		Findings: []Finding{
			{ID: "policy.missing", Severity: SeverityWarn},
		},
	}

	if report.Failed(false) {
		t.Fatal("default mode should not fail warnings")
	}
	if !report.Failed(true) {
		t.Fatal("strict mode should fail warnings")
	}
}

func TestLoadPolicyRejectsInvalidBlockedSourceGlob(t *testing.T) {
	project := t.TempDir()
	writePolicy(t, project, `
blocked_sources:
  - "client-["
`)

	_, _, _, err := LoadPolicy(project)
	if err == nil {
		t.Fatal("LoadPolicy() error = nil, want invalid glob error")
	}
}

func TestLoadPolicyRejectsInvalidBlockedPathGlob(t *testing.T) {
	project := t.TempDir()
	writePolicy(t, project, `
blocked_paths:
  - ".claude/agents/["
`)

	_, _, _, err := LoadPolicy(project)
	if err == nil {
		t.Fatal("LoadPolicy() error = nil, want invalid glob error")
	}
}

func TestBlockedPathMatchesDirectoryPrefix(t *testing.T) {
	if !blockedPathMatches(".claude/agents", ".claude/agents/reviewer.md") {
		t.Fatal("expected directory prefix blocked path match")
	}
	if blockedPathMatches(".claude/agents", ".claude/rules/reviewer.md") {
		t.Fatal("unexpected blocked path match")
	}
}

func TestInitPolicyCreatesDefaultPolicy(t *testing.T) {
	project := t.TempDir()

	policyPath, created, err := InitPolicy(project, InitOptions{})
	if err != nil {
		t.Fatalf("InitPolicy() error = %v", err)
	}
	if !created {
		t.Fatal("InitPolicy() created = false, want true")
	}
	if filepath.Base(policyPath) != PolicyFileName {
		t.Fatalf("policy path = %q, want %s", policyPath, PolicyFileName)
	}

	content, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("ReadFile(policy) error = %v", err)
	}
	if !strings.Contains(string(content), "required_sources: []") {
		t.Fatalf("default policy missing required_sources: %s", content)
	}

	if _, _, found, err := LoadPolicy(project); err != nil || !found {
		t.Fatalf("LoadPolicy() found=%v err=%v, want found with no error", found, err)
	}
}

func TestInitPolicyRefusesOverwriteWithoutForce(t *testing.T) {
	project := t.TempDir()
	writePolicy(t, project, "required_sources: []\n")

	if _, _, err := InitPolicy(project, InitOptions{}); err == nil {
		t.Fatal("InitPolicy() error = nil, want overwrite refusal")
	}

	policyPath, created, err := InitPolicy(project, InitOptions{Force: true})
	if err != nil {
		t.Fatalf("InitPolicy(force) error = %v", err)
	}
	if created {
		t.Fatal("InitPolicy(force) created = true, want false for overwrite")
	}
	content, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("ReadFile(policy) error = %v", err)
	}
	if !strings.Contains(string(content), "csaw project policy") {
		t.Fatalf("policy was not overwritten with default template: %s", content)
	}
}

func TestSourceMatchesGlob(t *testing.T) {
	if !sourceMatches("client-*", "client-acme") {
		t.Fatal("expected glob source match")
	}
	if sourceMatches("client-*", "personal") {
		t.Fatal("unexpected glob source match")
	}
}

func writeSourceConfig(t *testing.T, paths runtime.Paths, source sources.Source) {
	t.Helper()
	manager := sources.Manager{Paths: paths}
	if err := manager.Save(sources.Config{Sources: []sources.Source{source}}); err != nil {
		t.Fatalf("Save(source config) error = %v", err)
	}
}

func writePinState(t *testing.T, project string, pins ...pinning.Pin) {
	t.Helper()
	if err := pinning.Write(project, pinning.PinState{Pins: pins}); err != nil {
		t.Fatalf("pinning.Write() error = %v", err)
	}
}

func writePolicy(t *testing.T, project string, content string) {
	t.Helper()
	policyDir := filepath.Join(project, PolicyDirName)
	if err := os.MkdirAll(policyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(policy) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(policyDir, PolicyFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}
}

func writeMountedFile(t *testing.T, project string, relPath string, sourceName string, content string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), filepath.Base(relPath))
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	target := filepath.Join(project, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	mode := linkmode.Detect()
	if err := linkmode.Create(mode, source, target); err != nil {
		t.Fatalf("Create link error = %v", err)
	}

	state := workspace.MountState{
		Entries: []workspace.MountedStateEntry{
			{
				RelativePath: relPath,
				SourceName:   sourceName,
				SourcePath:   source,
			},
		},
	}
	if err := workspace.WriteMountState(project, state); err != nil {
		t.Fatalf("WriteMountState() error = %v", err)
	}
}

func assertFinding(t *testing.T, report Report, id string, severity Severity) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id && finding.Severity == severity {
			return
		}
	}
	t.Fatalf("finding %s/%s not found in %+v", id, severity, report.Findings)
}

func assertFindingDetail(t *testing.T, report Report, id string, detail string) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id && finding.Detail == detail {
			return
		}
	}
	t.Fatalf("finding %s with detail %q not found in %+v", id, detail, report.Findings)
}
