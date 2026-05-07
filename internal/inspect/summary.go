package inspect

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/pinning"
	"github.com/NicholasCullenCooper/csaw/internal/profiles"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

type Summary struct {
	ProjectRoot string
	Paths       runtime.Paths
	Sources     []sources.Source
	Mounted     []drift.Status
	Pins        []pinning.Pin
	// Protected maps source name → list of protected relative paths in that source.
	Protected map[string][]string
}

func BuildSummary(ctx context.Context, projectRoot string, paths runtime.Paths, manager sources.Manager) (Summary, error) {
	_ = ctx

	cfg, err := manager.Load()
	if err != nil {
		return Summary{}, err
	}

	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return Summary{}, err
	}

	var mounted []drift.Status
	if len(state.Entries) > 0 {
		mounted = drift.InspectMountState(projectRoot, state, linkmode.Detect())
	} else {
		links, err := workspace.FindMountedLinks(projectRoot, paths.Root)
		if err != nil {
			return Summary{}, err
		}
		mounted = drift.InspectLinks(links)
	}

	pinState, _ := pinning.Read(projectRoot)

	// Load per-source protected paths
	protected := map[string][]string{}
	if catalog, err := manager.ExistingCatalog(); err == nil {
		if resolver, err := profiles.NewCatalogResolver(paths, catalog); err == nil {
			for sourceName, policy := range resolver.Policies() {
				if sourceName == "" || len(policy.Protected) == 0 {
					continue
				}
				protected[sourceName] = append([]string(nil), policy.Protected...)
			}
		}
	}

	return Summary{
		ProjectRoot: projectRoot,
		Paths:       paths,
		Sources:     cfg.Sources,
		Mounted:     mounted,
		Pins:        pinState.Pins,
		Protected:   protected,
	}, nil
}

func RenderSummary(summary Summary) string {
	var b strings.Builder

	b.WriteString(output.Bold("csaw inspect"))
	b.WriteString("\n\n")

	// Project info
	writeLabel(&b, "project", summary.ProjectRoot)
	writeLabel(&b, "csaw home", summary.Paths.Root)
	writeLabel(&b, "sources", fmt.Sprintf("%d", len(summary.Sources)))
	writeLabel(&b, "mounted", fmt.Sprintf("%d", len(summary.Mounted)))

	if len(summary.Pins) > 0 {
		writeLabel(&b, "pinned", fmt.Sprintf("%d", len(summary.Pins)))
	}

	// Build pin lookup
	pinMap := make(map[string]string)
	for _, pin := range summary.Pins {
		pinMap[pin.Source] = pin.Ref
	}

	// Sources
	if len(summary.Sources) > 0 {
		b.WriteString("\n")
		b.WriteString(output.Bold("Sources"))
		b.WriteString("\n")
		for _, source := range summary.Sources {
			meta := string(source.Kind)
			if source.Priority != 0 {
				meta += fmt.Sprintf(", priority %d", source.Priority)
			}

			line := fmt.Sprintf("  %s %s %s %s",
				output.Accent(source.Name),
				output.Faint("("+meta+")"),
				output.Faint("→"),
				source.CheckoutPath(summary.Paths),
			)

			if ref, ok := pinMap[source.Name]; ok {
				line += " " + output.Warn("[pinned → "+ref+"]")
			}
			if protected, ok := summary.Protected[source.Name]; ok && len(protected) > 0 {
				line += " " + output.Warn(fmt.Sprintf("[%d protected]", len(protected)))
			}

			b.WriteString(line + "\n")

			// List protected files indented under the source
			if protected, ok := summary.Protected[source.Name]; ok {
				for _, p := range protected {
					b.WriteString(fmt.Sprintf("    %s %s\n", output.Warn("*"), output.Faint("protected: "+p)))
				}
			}
		}
	}

	// Mounted links grouped by source, then by kind within each source
	if len(summary.Mounted) > 0 {
		b.WriteString("\n")
		b.WriteString(output.Bold("Mounted files"))
		b.WriteString("\n")

		groups := groupBySource(summary.Mounted)
		for _, group := range groups {
			b.WriteString(fmt.Sprintf("\n  %s\n", output.Accent(group.name)))
			writeKindGroups(&b, group.statuses)
		}
	}

	return b.String()
}

func RenderSourceDetails(source sources.Source, paths runtime.Paths) (string, error) {
	var b strings.Builder

	b.WriteString(output.Bold("Source: " + source.Name))
	b.WriteString("\n\n")
	writeLabel(&b, "kind", string(source.Kind))
	if source.URL != "" {
		writeLabel(&b, "url", source.URL)
	}
	if source.Path != "" {
		writeLabel(&b, "path", source.Path)
	}
	writeLabel(&b, "checkout", source.CheckoutPath(paths))

	return b.String(), nil
}

func RenderMarkdownPreview(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if err != nil {
		return "", err
	}

	return renderer.Render(string(content))
}

// RenderMountResult formats the output of a mount operation.
func RenderMountResult(linked, stashed, skipped, alreadyLinked int, toolDirCount int) string {
	var parts []string

	if linked > 0 {
		parts = append(parts, output.Success(fmt.Sprintf("%d linked", linked)))
	}
	if alreadyLinked > 0 {
		parts = append(parts, output.Faint(fmt.Sprintf("%d already linked", alreadyLinked)))
	}
	if stashed > 0 {
		parts = append(parts, output.Warn(fmt.Sprintf("%d stashed", stashed)))
	}
	if skipped > 0 {
		parts = append(parts, output.Faint(fmt.Sprintf("%d skipped", skipped)))
	}

	result := strings.Join(parts, output.Faint(" · "))

	if toolDirCount > 0 {
		result += "\n" + output.Faint(fmt.Sprintf("  expanded into %d tool directories", toolDirCount))
	}

	return result
}

// RenderUnmountResult formats the output of an unmount operation.
func RenderUnmountResult(removed, restored int) string {
	var parts []string

	if removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", removed))
	}
	if restored > 0 {
		parts = append(parts, fmt.Sprintf("%d restored", restored))
	}

	return strings.Join(parts, output.Faint(" · "))
}

type sourceGroup struct {
	name     string
	statuses []drift.Status
}

// kindDisplayOrder is the canonical order kinds appear in the inspect output.
// Instructions come first because they're always loaded by the AI tool;
// agents next because they're the spawnable specialists; then skills, rules,
// MCP, then anything unclassified.
var kindDisplayOrder = []mount.Kind{
	mount.KindInstruction,
	mount.KindAgent,
	mount.KindSkill,
	mount.KindRule,
	mount.KindMCP,
	mount.KindOther,
}

func writeKindGroups(b *strings.Builder, statuses []drift.Status) {
	byKind := make(map[mount.Kind][]drift.Status)
	for _, status := range statuses {
		k := mount.KindOfProjectPath(status.RelativePath)
		byKind[k] = append(byKind[k], status)
	}

	for _, kind := range kindDisplayOrder {
		group, ok := byKind[kind]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("    %s %s\n",
			output.Faint(mount.KindLabel(kind)),
			output.Faint(fmt.Sprintf("(%d)", len(group))),
		))
		for _, status := range group {
			symbol := output.SymbolOK
			label := ""
			if !status.Healthy {
				symbol = output.SymbolWarn
				label = " " + output.Warn(status.Issue)
			}
			b.WriteString(fmt.Sprintf("      %s %s%s\n", symbol, status.RelativePath, label))
		}
	}
}

func groupBySource(statuses []drift.Status) []sourceGroup {
	order := make([]string, 0)
	groups := make(map[string][]drift.Status)

	for _, status := range statuses {
		name := status.SourceName
		if name == "" {
			name = "unknown"
		}
		if _, ok := groups[name]; !ok {
			order = append(order, name)
		}
		groups[name] = append(groups[name], status)
	}

	result := make([]sourceGroup, 0, len(order))
	for _, name := range order {
		result = append(result, sourceGroup{name: name, statuses: groups[name]})
	}
	return result
}

var labelKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(14)

func writeLabel(b *strings.Builder, key, value string) {
	b.WriteString(fmt.Sprintf("  %s %s\n", labelKeyStyle.Render(key+":"), value))
}
