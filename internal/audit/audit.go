package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/pinning"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

const (
	PolicyDirName  = ".csaw"
	PolicyFileName = "policy.yml"
)

const DefaultPolicyTemplate = `# csaw project policy
#
# Keep project-owned context in the repo. Use this file for context that must be
# composed, pinned, blocked, or audited as local state.

# Sources that must be mounted before work starts.
#
# A string checks only the source name:
#   - team
#
# An object can also require the configured source URL and project pin. The
# ref is the csaw project pin set by "csaw pin <source>@<ref>"; audit does not
# infer it from the source checkout's current branch.
required_sources: []
#  - name: client-acme
#    url: git@example.com:org/client-acme-ai.git
#    ref: main

# Sources that must not be active in this project. Glob patterns are supported.
blocked_sources: []
#  - other-client-*
#  - personal-experimental

# Artifact kinds that must not be mounted. Valid values:
# instructions, rules, agents, skills, mcp.
blocked_kinds: []
#  - mcp
#  - agents

# Mounted project paths that must not be active. Glob patterns are supported.
blocked_paths: []
#  - .mcp.json
#  - .claude/agents/**

# Artifact kinds that must be mounted. Valid values:
# instructions, rules, agents, skills, mcp.
required_kinds: []
#  - instructions
#  - rules
`

type Severity string

const (
	SeverityOK    Severity = "ok"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

type SourceRequirement struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

type Policy struct {
	RequiredSources []SourceRequirement `json:"required_sources,omitempty"`
	BlockedSources  []string            `json:"blocked_sources,omitempty"`
	BlockedKinds    []mount.Kind        `json:"blocked_kinds,omitempty"`
	BlockedPaths    []string            `json:"blocked_paths,omitempty"`
	RequiredKinds   []mount.Kind        `json:"required_kinds,omitempty"`
}

type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Source   string   `json:"source,omitempty"`
	Kind     string   `json:"kind,omitempty"`
	Path     string   `json:"path,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

type Report struct {
	ProjectRoot string    `json:"project_root"`
	PolicyPath  string    `json:"policy_path,omitempty"`
	PolicyFound bool      `json:"policy_found"`
	Mounted     int       `json:"mounted"`
	Findings    []Finding `json:"findings"`
}

type InitOptions struct {
	Force bool
}

func InitPolicy(projectRoot string, options InitOptions) (string, bool, error) {
	existing, found, err := ExistingPolicyPath(projectRoot)
	if err != nil {
		return "", false, err
	}
	if found && !options.Force {
		return "", false, fmt.Errorf("%s already exists; rerun with --force to overwrite", existing)
	}

	target := filepath.Join(projectRoot, PolicyDirName, PolicyFileName)
	created := true
	if found {
		target = existing
		created = false
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(target, []byte(DefaultPolicyTemplate), 0o644); err != nil {
		return "", false, err
	}
	return target, created, nil
}

func Run(projectRoot string, paths runtime.Paths) (Report, error) {
	policy, policyPath, policyFound, err := LoadPolicy(projectRoot)
	if err != nil {
		return Report{}, err
	}

	statuses, err := mountedStatuses(projectRoot, paths)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		ProjectRoot: projectRoot,
		PolicyPath:  policyPath,
		PolicyFound: policyFound,
		Mounted:     len(statuses),
	}

	if !policyFound {
		report.add(Finding{
			ID:       "policy.missing",
			Severity: SeverityWarn,
			Message:  "no project policy found",
			Path:     filepath.ToSlash(filepath.Join(PolicyDirName, PolicyFileName)),
			Detail:   "audit can check mount health, but required and blocked context are not declared",
		})
	} else {
		report.add(Finding{
			ID:       "policy.loaded",
			Severity: SeverityOK,
			Message:  "project policy loaded",
			Path:     policyPath,
		})
	}

	report.checkMountHealth(statuses)
	if policyFound {
		sourceIndex, pinState, err := loadSourceContext(projectRoot, paths, policy.RequiredSources)
		if err != nil {
			return Report{}, err
		}
		report.checkRequiredSources(policy.RequiredSources, statuses, sourceIndex, pinState)
		report.checkBlockedSources(policy.BlockedSources, statuses)
		report.checkBlockedKinds(policy.BlockedKinds, statuses)
		report.checkBlockedPaths(policy.BlockedPaths, statuses)
		report.checkRequiredKinds(policy.RequiredKinds, statuses)
	}

	return report, nil
}

func LoadPolicy(projectRoot string) (Policy, string, bool, error) {
	candidate, found, err := ExistingPolicyPath(projectRoot)
	if err != nil {
		return Policy{}, "", false, err
	}
	if !found {
		return Policy{}, "", false, nil
	}

	content, err := os.ReadFile(candidate)
	if err != nil {
		return Policy{}, "", false, err
	}

	policy, err := parsePolicy(content)
	if err != nil {
		return Policy{}, "", false, fmt.Errorf("%s: %w", candidate, err)
	}
	return policy, candidate, true, nil
}

func ExistingPolicyPath(projectRoot string) (string, bool, error) {
	candidates := []string{
		filepath.Join(projectRoot, PolicyDirName, PolicyFileName),
		filepath.Join(projectRoot, PolicyDirName, "policy.yaml"),
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, err
		}
		return candidate, true, nil
	}

	return "", false, nil
}

func parsePolicy(content []byte) (Policy, error) {
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(runtime.StripBOM(string(content))), &raw); err != nil {
		return Policy{}, err
	}

	var policy Policy
	var err error

	if value, ok := raw["required_sources"]; ok {
		policy.RequiredSources, err = normalizeSourceRequirements(value)
		if err != nil {
			return Policy{}, fmt.Errorf("required_sources: %w", err)
		}
	}

	if value, ok := raw["blocked_sources"]; ok {
		policy.BlockedSources, err = normalizeStringList(value)
		if err != nil {
			return Policy{}, fmt.Errorf("blocked_sources: %w", err)
		}
		for _, pattern := range policy.BlockedSources {
			if _, err := path.Match(pattern, ""); err != nil {
				return Policy{}, fmt.Errorf("blocked_sources: invalid pattern %q: %w", pattern, err)
			}
		}
	}

	if value, ok := raw["blocked_kinds"]; ok {
		policy.BlockedKinds, err = normalizeKindList("blocked_kinds", value)
		if err != nil {
			return Policy{}, err
		}
	}

	if value, ok := raw["blocked_paths"]; ok {
		policy.BlockedPaths, err = normalizePathPatterns(value)
		if err != nil {
			return Policy{}, fmt.Errorf("blocked_paths: %w", err)
		}
	}

	if value, ok := raw["required_kinds"]; ok {
		policy.RequiredKinds, err = normalizeKindList("required_kinds", value)
		if err != nil {
			return Policy{}, err
		}
	}

	return policy, nil
}

func normalizeKindList(field string, value any) ([]mount.Kind, error) {
	rawKinds, err := normalizeStringList(value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}

	kinds := make([]mount.Kind, 0, len(rawKinds))
	for _, rawKind := range rawKinds {
		kind, err := mount.ParseKind(rawKind)
		if err != nil {
			return nil, err
		}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

func normalizePathPatterns(value any) ([]string, error) {
	patterns, err := normalizeStringList(value)
	if err != nil {
		return nil, err
	}

	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = runtime.NormalizeRegistryPath(pattern)
		if pattern == "" || pattern == "." {
			return nil, errors.New("path pattern cannot be empty")
		}
		if _, err := doublestar.PathMatch(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		normalized = append(normalized, pattern)
	}
	return normalized, nil
}

func normalizeSourceRequirements(value any) ([]SourceRequirement, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, errors.New("expected a list")
	}

	requirements := make([]SourceRequirement, 0, len(list))
	for _, item := range list {
		switch typed := item.(type) {
		case string:
			name := strings.TrimSpace(typed)
			if name == "" {
				return nil, errors.New("source name cannot be empty")
			}
			requirements = append(requirements, SourceRequirement{Name: name})
		case map[string]any:
			requirement, err := normalizeSourceRequirementMap(typed)
			if err != nil {
				return nil, err
			}
			requirements = append(requirements, requirement)
		case map[any]any:
			converted := make(map[string]any, len(typed))
			for key, value := range typed {
				text, ok := key.(string)
				if !ok {
					return nil, errors.New("source requirement field names must be strings")
				}
				converted[text] = value
			}
			requirement, err := normalizeSourceRequirementMap(converted)
			if err != nil {
				return nil, err
			}
			requirements = append(requirements, requirement)
		default:
			return nil, errors.New("expected source names or objects")
		}
	}

	return requirements, nil
}

func normalizeSourceRequirementMap(value map[string]any) (SourceRequirement, error) {
	var requirement SourceRequirement
	for key, raw := range value {
		text, ok := raw.(string)
		if !ok {
			return SourceRequirement{}, fmt.Errorf("%s must be a string", key)
		}
		switch key {
		case "name":
			requirement.Name = strings.TrimSpace(text)
		case "url":
			requirement.URL = strings.TrimSpace(text)
		case "ref":
			requirement.Ref = strings.TrimSpace(text)
		default:
			return SourceRequirement{}, fmt.Errorf("unknown source requirement field %q", key)
		}
	}
	if requirement.Name == "" {
		return SourceRequirement{}, errors.New("source requirement missing name")
	}
	return requirement, nil
}

func normalizeStringList(value any) ([]string, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, errors.New("expected a list")
	}

	values := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, errors.New("expected list items to be strings")
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, errors.New("list items cannot be empty")
		}
		values = append(values, text)
	}
	return values, nil
}

func mountedStatuses(projectRoot string, paths runtime.Paths) ([]drift.Status, error) {
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return nil, err
	}
	if len(state.Entries) > 0 {
		return drift.InspectMountState(projectRoot, state, linkmode.Detect()), nil
	}

	links, err := workspace.FindMountedLinks(projectRoot, paths.Root)
	if err != nil {
		return nil, err
	}
	return drift.InspectLinks(links), nil
}

func (r *Report) checkMountHealth(statuses []drift.Status) {
	if len(statuses) == 0 {
		r.add(Finding{
			ID:       "mount.none",
			Severity: SeverityWarn,
			Message:  "no active csaw context found",
			Detail:   "mount a profile before auditing required or blocked context",
		})
		return
	}

	unhealthy := 0
	for _, status := range statuses {
		if status.Healthy {
			continue
		}
		unhealthy++
		r.add(Finding{
			ID:       "mount.unhealthy",
			Severity: SeverityError,
			Message:  "mounted file needs attention",
			Source:   status.SourceName,
			Path:     status.RelativePath,
			Detail:   status.Issue,
		})
	}
	if unhealthy == 0 {
		r.add(Finding{
			ID:       "mount.healthy",
			Severity: SeverityOK,
			Message:  fmt.Sprintf("%d mounted file(s) healthy", len(statuses)),
		})
	}
}

func loadSourceContext(projectRoot string, paths runtime.Paths, requirements []SourceRequirement) (map[string]sources.Source, pinning.PinState, error) {
	sourceIndex := map[string]sources.Source{}
	if sourceRequirementsNeedURL(requirements) {
		manager := sources.Manager{Paths: paths}
		cfg, err := manager.Load()
		if err != nil {
			return nil, pinning.PinState{}, err
		}
		sourceIndex = make(map[string]sources.Source, len(cfg.Sources))
		for _, source := range cfg.Sources {
			sourceIndex[source.Name] = source
		}
	}

	pinState, err := pinning.Read(projectRoot)
	if err != nil {
		return nil, pinning.PinState{}, err
	}

	return sourceIndex, pinState, nil
}

func sourceRequirementsNeedURL(requirements []SourceRequirement) bool {
	for _, requirement := range requirements {
		if requirement.URL != "" {
			return true
		}
	}
	return false
}

func (r *Report) checkRequiredSources(requirements []SourceRequirement, statuses []drift.Status, sourceIndex map[string]sources.Source, pinState pinning.PinState) {
	active := healthySources(statuses)
	for _, requirement := range requirements {
		r.checkRequiredSource(requirement, active, sourceIndex, pinState)
	}
}

func (r *Report) checkRequiredSource(requirement SourceRequirement, active map[string]bool, sourceIndex map[string]sources.Source, pinState pinning.PinState) {
	if !active[requirement.Name] {
		r.add(Finding{
			ID:       "source.required.missing",
			Severity: SeverityError,
			Message:  fmt.Sprintf("required source %q is not active", requirement.Name),
			Source:   requirement.Name,
		})
		return
	}

	r.add(Finding{
		ID:       "source.required.present",
		Severity: SeverityOK,
		Message:  fmt.Sprintf("required source %q is active", requirement.Name),
		Source:   requirement.Name,
	})

	if requirement.URL != "" {
		source, ok := sourceIndex[requirement.Name]
		if !ok {
			r.add(Finding{
				ID:       "source.required.metadata_missing",
				Severity: SeverityError,
				Message:  fmt.Sprintf("required source %q is active but not configured", requirement.Name),
				Source:   requirement.Name,
				Detail:   "cannot verify required url",
			})
		} else if source.URL != requirement.URL {
			r.add(Finding{
				ID:       "source.required.url_mismatch",
				Severity: SeverityError,
				Message:  fmt.Sprintf("required source %q URL does not match policy", requirement.Name),
				Source:   requirement.Name,
				Detail:   fmt.Sprintf("expected %s, configured %s", requirement.URL, source.URL),
			})
		} else {
			r.add(Finding{
				ID:       "source.required.url_match",
				Severity: SeverityOK,
				Message:  fmt.Sprintf("required source %q URL matches policy", requirement.Name),
				Source:   requirement.Name,
			})
		}
	}

	if requirement.Ref != "" {
		ref, ok := pinning.Get(pinState, requirement.Name)
		if !ok {
			r.add(Finding{
				ID:       "source.required.ref_mismatch",
				Severity: SeverityError,
				Message:  fmt.Sprintf("required source %q pin does not match policy", requirement.Name),
				Source:   requirement.Name,
				Detail:   fmt.Sprintf("expected %s, actual unpinned", requirement.Ref),
			})
		} else if ref != requirement.Ref {
			r.add(Finding{
				ID:       "source.required.ref_mismatch",
				Severity: SeverityError,
				Message:  fmt.Sprintf("required source %q pin does not match policy", requirement.Name),
				Source:   requirement.Name,
				Detail:   fmt.Sprintf("expected %s, actual %s", requirement.Ref, ref),
			})
		} else {
			r.add(Finding{
				ID:       "source.required.ref_match",
				Severity: SeverityOK,
				Message:  fmt.Sprintf("required source %q pin matches policy", requirement.Name),
				Source:   requirement.Name,
			})
		}
	}
}

func (r *Report) checkBlockedSources(patterns []string, statuses []drift.Status) {
	if len(patterns) == 0 {
		return
	}

	active := allSources(statuses)
	blocked := make([]string, 0)
	for source := range active {
		if source == "" {
			continue
		}
		for _, pattern := range patterns {
			if sourceMatches(pattern, source) {
				blocked = append(blocked, source)
				r.add(Finding{
					ID:       "source.blocked.active",
					Severity: SeverityError,
					Message:  fmt.Sprintf("blocked source %q is active", source),
					Source:   source,
					Detail:   "matched " + pattern,
				})
				break
			}
		}
	}

	if len(blocked) == 0 {
		r.add(Finding{
			ID:       "source.blocked.clear",
			Severity: SeverityOK,
			Message:  "no blocked sources are active",
		})
	}
}

func (r *Report) checkBlockedKinds(kinds []mount.Kind, statuses []drift.Status) {
	if len(kinds) == 0 {
		return
	}

	blocked := make(map[mount.Kind]bool, len(kinds))
	for _, kind := range kinds {
		blocked[kind] = true
	}

	var active []mount.Kind
	seen := map[mount.Kind]bool{}
	for _, status := range statuses {
		if !status.Healthy {
			continue
		}
		kind := mount.KindOfProjectPath(status.RelativePath)
		if !blocked[kind] || seen[kind] {
			continue
		}
		seen[kind] = true
		active = append(active, kind)
		r.add(Finding{
			ID:       "kind.blocked.active",
			Severity: SeverityError,
			Message:  fmt.Sprintf("blocked kind %q is active", mount.KindLabel(kind)),
			Kind:     string(kind),
		})
	}

	if len(active) == 0 {
		r.add(Finding{
			ID:       "kind.blocked.clear",
			Severity: SeverityOK,
			Message:  "no blocked kinds are active",
		})
	}
}

func (r *Report) checkBlockedPaths(patterns []string, statuses []drift.Status) {
	if len(patterns) == 0 {
		return
	}

	blocked := 0
	for _, status := range statuses {
		if !status.Healthy {
			continue
		}
		for _, pattern := range patterns {
			if blockedPathMatches(pattern, status.RelativePath) {
				blocked++
				r.add(Finding{
					ID:       "path.blocked.active",
					Severity: SeverityError,
					Message:  fmt.Sprintf("blocked path %q is active", status.RelativePath),
					Source:   status.SourceName,
					Path:     status.RelativePath,
					Detail:   "matched " + pattern,
				})
				break
			}
		}
	}

	if blocked == 0 {
		r.add(Finding{
			ID:       "path.blocked.clear",
			Severity: SeverityOK,
			Message:  "no blocked paths are active",
		})
	}
}

func (r *Report) checkRequiredKinds(kinds []mount.Kind, statuses []drift.Status) {
	if len(kinds) == 0 {
		return
	}

	active := healthyKinds(statuses)
	for _, kind := range kinds {
		if active[kind] {
			r.add(Finding{
				ID:       "kind.required.present",
				Severity: SeverityOK,
				Message:  fmt.Sprintf("required kind %q is active", mount.KindLabel(kind)),
				Kind:     string(kind),
			})
			continue
		}
		r.add(Finding{
			ID:       "kind.required.missing",
			Severity: SeverityError,
			Message:  fmt.Sprintf("required kind %q is not active", mount.KindLabel(kind)),
			Kind:     string(kind),
		})
	}
}

func healthySources(statuses []drift.Status) map[string]bool {
	sources := make(map[string]bool)
	for _, status := range statuses {
		if status.Healthy && status.SourceName != "" {
			sources[status.SourceName] = true
		}
	}
	return sources
}

func allSources(statuses []drift.Status) map[string]bool {
	sources := make(map[string]bool)
	for _, status := range statuses {
		if status.SourceName != "" {
			sources[status.SourceName] = true
		}
	}
	return sources
}

func healthyKinds(statuses []drift.Status) map[mount.Kind]bool {
	kinds := make(map[mount.Kind]bool)
	for _, status := range statuses {
		if status.Healthy {
			kinds[mount.KindOfProjectPath(status.RelativePath)] = true
		}
	}
	return kinds
}

func sourceMatches(pattern, source string) bool {
	if pattern == source {
		return true
	}
	matched, err := path.Match(pattern, source)
	return err == nil && matched
}

func blockedPathMatches(pattern string, relPath string) bool {
	pattern = runtime.NormalizeRegistryPath(pattern)
	relPath = runtime.NormalizeRegistryPath(relPath)
	if pattern == relPath {
		return true
	}
	if !hasPathGlob(pattern) && strings.HasPrefix(relPath, pattern+"/") {
		return true
	}
	matched, err := doublestar.PathMatch(pattern, relPath)
	return err == nil && matched
}

func hasPathGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func (r *Report) add(finding Finding) {
	r.Findings = append(r.Findings, finding)
}

func (r Report) Failed(strict bool) bool {
	for _, finding := range r.Findings {
		if finding.Severity == SeverityError {
			return true
		}
		if strict && finding.Severity == SeverityWarn {
			return true
		}
	}
	return false
}

func (r Report) Counts() (ok int, warn int, fail int) {
	for _, finding := range r.Findings {
		switch finding.Severity {
		case SeverityOK:
			ok++
		case SeverityWarn:
			warn++
		case SeverityError:
			fail++
		}
	}
	return ok, warn, fail
}

func (r Report) FailureSummary(strict bool) string {
	_, warnings, errors := r.Counts()
	if errors > 0 {
		return fmt.Sprintf("audit found %d error(s)", errors)
	}
	if strict && warnings > 0 {
		return fmt.Sprintf("audit found %d warning(s) in strict mode", warnings)
	}
	return ""
}

func RenderText(report Report) string {
	var b strings.Builder

	b.WriteString(output.Bold("csaw audit"))
	b.WriteString("\n\n")
	writeLabel(&b, "project", report.ProjectRoot)
	if report.PolicyFound {
		writeLabel(&b, "policy", report.PolicyPath)
	} else {
		writeLabel(&b, "policy", output.Faint("not found"))
	}
	writeLabel(&b, "mounted", fmt.Sprintf("%d", report.Mounted))

	findings := append([]Finding(nil), report.Findings...)
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
	})

	if len(findings) > 0 {
		b.WriteString("\n")
		b.WriteString(output.Bold("Findings"))
		b.WriteString("\n")
		for _, finding := range findings {
			b.WriteString("  ")
			b.WriteString(symbolFor(finding.Severity))
			b.WriteString(" ")
			b.WriteString(finding.ID)
			b.WriteString(" ")
			b.WriteString(finding.Message)
			if finding.Path != "" {
				b.WriteString(" ")
				b.WriteString(output.Faint("[" + finding.Path + "]"))
			}
			if finding.Detail != "" {
				b.WriteString(" ")
				b.WriteString(output.Faint("(" + finding.Detail + ")"))
			}
			b.WriteString("\n")
		}
	}

	ok, warn, fail := report.Counts()
	b.WriteString("\n")
	writeLabel(&b, "summary", fmt.Sprintf("%d ok · %d warn · %d error", ok, warn, fail))
	return b.String()
}

func RenderJSON(report Report) ([]byte, error) {
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func writeLabel(b *strings.Builder, key, value string) {
	b.WriteString(fmt.Sprintf("%-12s %s\n", output.Faint(key+":"), value))
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 3
	case SeverityWarn:
		return 2
	case SeverityOK:
		return 1
	default:
		return 0
	}
}

func symbolFor(severity Severity) string {
	switch severity {
	case SeverityError:
		return output.SymbolErr
	case SeverityWarn:
		return output.SymbolWarn
	default:
		return output.SymbolOK
	}
}
