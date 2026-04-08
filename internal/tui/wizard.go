package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StepKind identifies the type of wizard step.
type StepKind int

const (
	StepSelect  StepKind = iota // Choose from a list
	StepInput                   // Free-text input
	StepConfirm                 // Yes/No
)

// Step defines a single step in a wizard flow.
type Step struct {
	Kind        StepKind
	Key         string // Result key (e.g., "source_name")
	Title       string
	Description string
	Options     []PickerItem // For StepSelect
	Placeholder string       // For StepInput
	Default     string       // For StepInput or StepConfirm ("y"/"n")
}

// WizardResult holds the collected values from all steps.
type WizardResult struct {
	Values  map[string]string
	Aborted bool
}

type wizardModel struct {
	steps   []Step
	current int
	results map[string]string
	aborted bool
	width   int
	height  int

	// Step-specific state
	selectCursor int
	selectFilter string
	filtered     []PickerItem
	textInput    textinput.Model
	confirmValue bool
}

var (
	wizTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14")).
			MarginBottom(1)

	wizDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	wizStepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	wizBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(1, 2)

	wizSelectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	wizUnselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	wizConfirmStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	wizHelpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginTop(1)
)

// RunWizard runs a multi-step wizard and returns the collected results.
func RunWizard(steps []Step) (WizardResult, error) {
	if len(steps) == 0 {
		return WizardResult{Values: map[string]string{}}, nil
	}

	m := wizardModel{
		steps:   steps,
		results: make(map[string]string),
	}
	m.initStep()

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return WizardResult{}, err
	}

	fm := final.(wizardModel)
	return WizardResult{
		Values:  fm.results,
		Aborted: fm.aborted,
	}, nil
}

func (m *wizardModel) initStep() {
	step := m.steps[m.current]

	switch step.Kind {
	case StepSelect:
		m.selectCursor = 0
		m.selectFilter = ""
		m.filtered = step.Options
	case StepInput:
		ti := textinput.New()
		ti.Placeholder = step.Placeholder
		if step.Default != "" {
			ti.SetValue(step.Default)
			ti.CursorEnd()
		}
		ti.Focus()
		ti.CharLimit = 256
		m.textInput = ti
	case StepConfirm:
		m.confirmValue = step.Default != "n"
	}
}

func (m wizardModel) Init() tea.Cmd {
	step := m.steps[m.current]
	if step.Kind == StepInput {
		return textinput.Blink
	}
	return nil
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.aborted = true
			return m, tea.Quit
		}
	}

	step := m.steps[m.current]
	switch step.Kind {
	case StepSelect:
		return m.updateSelect(msg)
	case StepInput:
		return m.updateInput(msg)
	case StepConfirm:
		return m.updateConfirm(msg)
	}

	return m, nil
}

func (m wizardModel) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if len(m.filtered) > 0 && m.selectCursor < len(m.filtered) {
				m.results[m.steps[m.current].Key] = m.filtered[m.selectCursor].Name
				return m.advance()
			}
		case "up", "ctrl+p":
			if m.selectCursor > 0 {
				m.selectCursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.selectCursor < len(m.filtered)-1 {
				m.selectCursor++
			}
			return m, nil
		}
	}
	return m, nil
}

func (m wizardModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "enter" {
			value := strings.TrimSpace(m.textInput.Value())
			if value == "" && m.steps[m.current].Default != "" {
				value = m.steps[m.current].Default
			}
			if value != "" {
				m.results[m.steps[m.current].Key] = value
				return m.advance()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y":
			m.confirmValue = true
			m.results[m.steps[m.current].Key] = "y"
			return m.advance()
		case "n", "N":
			m.confirmValue = false
			m.results[m.steps[m.current].Key] = "n"
			return m.advance()
		case "enter":
			if m.confirmValue {
				m.results[m.steps[m.current].Key] = "y"
			} else {
				m.results[m.steps[m.current].Key] = "n"
			}
			return m.advance()
		case "left", "right", "tab", "h", "l":
			m.confirmValue = !m.confirmValue
			return m, nil
		}
	}
	return m, nil
}

func (m wizardModel) advance() (tea.Model, tea.Cmd) {
	m.current++
	if m.current >= len(m.steps) {
		return m, tea.Quit
	}
	m.initStep()
	if m.steps[m.current].Kind == StepInput {
		return m, textinput.Blink
	}
	return m, nil
}

func (m wizardModel) View() string {
	// Bubbletea calls View() one final time after tea.Quit — guard against
	// m.current being past the end of the steps slice.
	if m.current >= len(m.steps) {
		return ""
	}
	step := m.steps[m.current]

	var content strings.Builder

	// Step indicator
	if len(m.steps) > 1 {
		content.WriteString(wizStepStyle.Render(
			fmt.Sprintf("step %d of %d", m.current+1, len(m.steps)),
		))
		content.WriteString("\n")
	}

	// Title
	content.WriteString(wizTitleStyle.Render(step.Title))
	content.WriteString("\n")

	// Description
	if step.Description != "" {
		content.WriteString(wizDescStyle.Render(step.Description))
		content.WriteString("\n\n")
	}

	// Step content
	switch step.Kind {
	case StepSelect:
		content.WriteString(m.viewSelect())
	case StepInput:
		content.WriteString(m.textInput.View())
		content.WriteString("\n")
	case StepConfirm:
		content.WriteString(m.viewConfirm())
	}

	// Help
	var help string
	switch step.Kind {
	case StepSelect:
		help = "↑↓ navigate · enter select · esc cancel"
	case StepInput:
		help = "enter confirm · esc cancel"
	case StepConfirm:
		help = "y/n or ←→ toggle · enter confirm · esc cancel"
	}
	content.WriteString(wizHelpStyle.Render(help))

	// Wrap in box
	boxWidth := 52
	if m.width > 0 && m.width < boxWidth+6 {
		boxWidth = m.width - 6
	}

	return wizBoxStyle.Width(boxWidth).Render(content.String()) + "\n"
}

func (m wizardModel) viewSelect() string {
	var b strings.Builder

	maxVisible := 8
	if m.height > 0 {
		maxVisible = max(4, m.height-12)
	}

	start := 0
	if m.selectCursor >= maxVisible {
		start = m.selectCursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.filtered))

	for i := start; i < end; i++ {
		item := m.filtered[i]
		cursor := "  "
		style := wizUnselectedStyle
		if i == m.selectCursor {
			cursor = wizSelectedStyle.Render("▸ ")
			style = wizSelectedStyle
		}

		line := fmt.Sprintf("%s%s", cursor, style.Render(item.Name))
		if item.Description != "" {
			line += " " + wizDescStyle.Render(item.Description)
		}
		b.WriteString(line + "\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(wizDescStyle.Render("  no options available") + "\n")
	}

	return b.String()
}

func (m wizardModel) viewConfirm() string {
	yes := "  Yes  "
	no := "  No  "

	if m.confirmValue {
		yes = wizConfirmStyle.Render("▸ Yes ")
		no = wizUnselectedStyle.Render("  No  ")
	} else {
		yes = wizUnselectedStyle.Render("  Yes ")
		no = wizConfirmStyle.Render("▸ No  ")
	}

	return yes + "    " + no + "\n"
}
