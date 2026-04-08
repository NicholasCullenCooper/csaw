package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	panelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(1, 2)

	panelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14"))

	panelSuccess = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	panelMuted = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	panelFile = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	panelHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
)

// MountPanel renders a styled summary of a mount operation.
func MountPanel(files []string, sources []string, stats string, hints []string) string {
	var b strings.Builder

	b.WriteString(panelTitle.Render("mounted"))
	b.WriteString("\n\n")

	// Group files by source
	if len(sources) == 1 {
		b.WriteString(panelSuccess.Render(sources[0]))
		b.WriteString("\n")
		for _, f := range files {
			b.WriteString(fmt.Sprintf(" %s %s\n", panelSuccess.Render("✔"), panelFile.Render(f)))
		}
	} else {
		for _, f := range files {
			b.WriteString(fmt.Sprintf(" %s %s\n", panelSuccess.Render("✔"), panelFile.Render(f)))
		}
	}

	if stats != "" {
		b.WriteString("\n")
		b.WriteString(panelMuted.Render(stats))
		b.WriteString("\n")
	}

	if len(hints) > 0 {
		b.WriteString("\n")
		for _, hint := range hints {
			b.WriteString(panelHint.Render(hint))
			b.WriteString("\n")
		}
	}

	return panelBorder.Render(b.String())
}

// ResultPanel renders a generic styled result panel.
func ResultPanel(title string, lines []string, hints []string) string {
	var b strings.Builder

	b.WriteString(panelTitle.Render(title))
	b.WriteString("\n\n")

	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(hints) > 0 {
		b.WriteString("\n")
		for _, hint := range hints {
			b.WriteString(panelHint.Render(hint))
			b.WriteString("\n")
		}
	}

	return panelBorder.Render(b.String())
}

// HintLine formats a "Next: command" style hint.
func HintLine(label, command string) string {
	return fmt.Sprintf("%s %s", panelMuted.Render(label), panelFile.Render(command))
}
