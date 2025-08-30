package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Preflight is rendered via viewport header/content/footer.

func displayPreflightItem(it PreflightItem) string {
	name := it.Story.Name
	if name == "" {
		name = it.Story.Slug
	}
	slug := "(" + it.Story.FullSlug + ")"
	if !it.Selected || it.State == StateSkip {
		name = subtleStyle.Render(name)
		slug = subtleStyle.Render(slug)
	}
	sym := storyTypeSymbol(it.Story)
	return fmt.Sprintf("%s %s  %s", sym, name, slug)
}

func (m *Model) updatePreflightViewport() {
	content := m.renderPreflightContent()
	m.viewport.SetContent(content)
}

func (m Model) renderPreflightHeader() string {
	total := len(m.preflight.items)
	collisions := 0
	for _, it := range m.preflight.items {
		if it.Collision {
			collisions++
		}
	}
	return fmt.Sprintf("Preflight – %d Items  |  Kollisionen: %d", total, collisions)
}

func (m Model) renderPreflightContent() string {
	var b strings.Builder
	total := len(m.preflight.items)

	if total == 0 {
		b.WriteString(warnStyle.Render("Keine Stories markiert.") + "\n")
		return b.String()
	}

	// Build stories slice in preflight visible order (shared helper)
	stories, order := m.visibleOrderPreflight()
	lines := generateTreeLinesFromStories(stories)
	for visPos, idx := range order {
		if visPos >= len(lines) {
			break
		}
		it := m.preflight.items[idx]
		content := lines[visPos]
		if it.Collision {
			content = collisionSign + " " + content
		} else {
			content = "  " + content
		}
		lineStyle := lipgloss.NewStyle().Width(m.width - 4)
		if visPos == m.preflight.listIndex {
			lineStyle = cursorLineStyle.Copy().Width(m.width - 4)
		}
		if strings.ToLower(it.State) == StateSkip {
			lineStyle = lineStyle.Faint(true)
		}
		content = lineStyle.Render(content)
		cursorCell := " "
		if visPos == m.preflight.listIndex {
			cursorCell = cursorBarStyle.Render(" ")
		}
		stateCell := " "
		switch it.Run {
		case RunRunning:
			stateCell = m.spinner.View()
		case RunDone:
			stateCell = stateDoneStyle.Render(stateLabel(it.State))
		case RunCancelled:
			stateCell = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("0")).Bold(true).Render("X")
		default:
			if it.State != "" {
				if st, ok := stateStyles[strings.ToLower(it.State)]; ok {
					stateCell = st.Render(stateLabel(it.State))
				} else {
					stateCell = it.State
				}
			}
		}
		lines[visPos] = cursorCell + stateCell + content
	}
	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

func (m Model) renderPreflightFooter() string {
	var statusLine string
	if m.syncing {
		statusLine = renderProgress(m.syncIndex, len(m.preflight.items), m.width-2)
	}

	var helpText string
	if m.syncing {
		helpText = "Syncing... | Ctrl+C to cancel"
	} else {
		helpText = "j/k bewegen  |  x skip  |  X alle skippen  |  c Skips entfernen  |  Enter OK  |  esc/q zurück"
	}

	return renderFooter(statusLine, helpText)
}

func renderProgress(done, total, width int) string {
	if total <= 0 {
		return ""
	}
	if width <= 0 {
		width = 20
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + fmt.Sprintf("] %d/%d", done, total)
}
