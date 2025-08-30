package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- UI Styles ---
var (
	titleStyle      = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("#8942E1"))
	subtitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3AC4BA")).Italic(true)
	subtleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))
	warnStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
	errorStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8942E1")).
			Padding(1, 2).
			Margin(1, 0)
	centeredStyle   = lipgloss.NewStyle().Align(lipgloss.Center)
	listHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8942E1")).
			Margin(0, 0, 1, 0)
	spaceItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)
	spaceSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#8942E1")).
				Padding(0, 1)
	dividerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	focusStyle       = lipgloss.NewStyle().Bold(true)
	cursorLineStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#2A2B3D"))
	cursorBarStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#FFAB78"))
	markBarStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#3AC4BA"))
	markNestedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3AC4BA"))
	collisionSign    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("!")
	stateCreateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stateUpdateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stateSkipStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	stateDoneStyle   = lipgloss.NewStyle().Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")).Bold(true)

	// markers for different story types (colored squares)
	symbolStory  = fgSymbol("#8942E1", "S")
	symbolFolder = fgSymbol("#3AC4BA", "F")
	symbolRoot   = fgSymbol("214", "R")
)

var stateStyles = map[string]lipgloss.Style{
	StateCreate: stateCreateStyle,
	StateUpdate: stateUpdateStyle,
	StateSkip:   stateSkipStyle,
}

// stateLabel renders the compact label for a textual state
func stateLabel(state string) string {
	switch strings.ToLower(state) {
	case StateCreate:
		return "C"
	case StateUpdate:
		return "U"
	case StateSkip:
		return "S"
	default:
		return state
	}
}

func fgSymbol(col, ch string) string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Render(ch)
	const reset = "\x1b[0m"
	return strings.TrimSuffix(s, reset) + "\x1b[39m"
}

// renderFooter creates a consistent footer across all views
// statusLine: optional status information (shown in subtleStyle)
// helpLines: help text lines (shown in helpStyle)
func renderFooter(statusLine string, helpLines ...string) string {
	var b strings.Builder

	if statusLine != "" {
		b.WriteString(subtleStyle.Render(statusLine) + "\n")
	}

	for _, line := range helpLines {
		b.WriteString(helpStyle.Render(line) + "\n")
	}

	// Remove trailing newline
	result := b.String()
	return strings.TrimSuffix(result, "\n")
}
