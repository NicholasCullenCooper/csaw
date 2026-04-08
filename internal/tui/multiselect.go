package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MultiSelectItem represents a toggleable item.
type MultiSelectItem struct {
	Key         string // Internal key (e.g., "claude")
	Label       string // Display label (e.g., "Claude Code")
	Description string // Optional description
	Selected    bool   // Initial selection state
}

// MultiSelectResult holds the selected keys.
type MultiSelectResult struct {
	Selected []string
	Aborted  bool
}

type multiSelectModel struct {
	title   string
	items   []MultiSelectItem
	cursor  int
	aborted bool
	width   int
	height  int
}

var (
	msCheckStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	msUncheckStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	msCursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	msLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	msDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	msTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).MarginBottom(1)
	msHelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginTop(1)
	msBoxStyle     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(1, 2)
)

// RunMultiSelect shows an interactive multi-select and returns the selected keys.
func RunMultiSelect(title string, items []MultiSelectItem) (MultiSelectResult, error) {
	if len(items) == 0 {
		return MultiSelectResult{}, nil
	}

	m := multiSelectModel{
		title: title,
		items: make([]MultiSelectItem, len(items)),
	}
	copy(m.items, items)

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return MultiSelectResult{}, err
	}

	fm := final.(multiSelectModel)
	if fm.aborted {
		return MultiSelectResult{Aborted: true}, nil
	}

	var selected []string
	for _, item := range fm.items {
		if item.Selected {
			selected = append(selected, item.Key)
		}
	}
	return MultiSelectResult{Selected: selected}, nil
}

func (m multiSelectModel) Init() tea.Cmd { return nil }

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			return m, tea.Quit
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j", "ctrl+n":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ", "x":
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
		case "a":
			allSelected := true
			for _, item := range m.items {
				if !item.Selected {
					allSelected = false
					break
				}
			}
			for i := range m.items {
				m.items[i].Selected = !allSelected
			}
		}
	}
	return m, nil
}

func (m multiSelectModel) View() string {
	var b strings.Builder

	b.WriteString(msTitleStyle.Render(m.title))
	b.WriteString("\n")

	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = msCursorStyle.Render("▸ ")
		}

		check := msUncheckStyle.Render("○")
		if item.Selected {
			check = msCheckStyle.Render("●")
		}

		label := msLabelStyle.Render(item.Label)
		if i == m.cursor {
			label = msCursorStyle.Render(item.Label)
		}

		line := fmt.Sprintf("%s%s %s", cursor, check, label)
		if item.Description != "" {
			line += " " + msDescStyle.Render(item.Description)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(msHelpStyle.Render("↑↓ navigate · space toggle · a all · enter confirm"))

	boxWidth := 52
	if m.width > 0 && m.width < boxWidth+6 {
		boxWidth = m.width - 6
	}

	return msBoxStyle.Width(boxWidth).Render(b.String()) + "\n"
}
