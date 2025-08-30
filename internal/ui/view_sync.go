package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderSyncHeader() string {
	var b strings.Builder

	// Count states for progress information
	total := len(m.preflight.items)
	completed := 0
	running := 0
	cancelled := 0
	pending := 0

	for _, item := range m.preflight.items {
		switch item.Run {
		case RunDone:
			completed++
		case RunRunning:
			running++
		case RunCancelled:
			cancelled++
		default: // RunPending
			pending++
		}
	}

	// Progress information with detailed counts
	statusParts := []string{fmt.Sprintf("%d✓", completed)}
	if running > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d◯", running))
	}
	if cancelled > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d✗", cancelled))
	}
	if pending > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d◦", pending))
	}

	progressText := fmt.Sprintf("Synchronisierung (%s von %d)", strings.Join(statusParts, " "), total)
	b.WriteString(listHeaderStyle.Render(progressText))
	b.WriteString("\n")

	// Current item being processed - find the running item
	for i, item := range m.preflight.items {
		if item.Run == RunRunning {
			itemText := fmt.Sprintf("Läuft: %s | %s (%s)", item.Story.Name, item.Story.FullSlug, item.State)
			b.WriteString(warnStyle.Render(itemText))
			break
		} else if i == m.syncIndex && m.syncing && item.Run == RunPending {
			// Fallback to sync index if no running item found
			itemText := fmt.Sprintf("Verarbeite: %s | %s (%s)", item.Story.Name, item.Story.FullSlug, item.State)
			b.WriteString(subtleStyle.Render(itemText))
			break
		}
	}

	b.WriteString("\n")
	return b.String()
}

func (m Model) renderSyncFooter() string {
	statusLine := ""
	if m.syncing {
		statusLine = m.spinner.View() + " Synchronisiere..."
	}

	help := "ctrl+c: Abbrechen"
	// If paused (not syncing) and there are pending items, offer resume hint
	if !m.syncing {
		hasPending := false
		for _, it := range m.preflight.items {
			if it.Run == RunPending {
				hasPending = true
				break
			}
		}
		if hasPending {
			help = "r: Fortsetzen  |  ctrl+c: Abbrechen"
		}
	}
	return renderFooter(statusLine, help)
}

func (m *Model) updateSyncViewport() {
	var content strings.Builder

	// Show sync progress
	total := len(m.preflight.items)
	completed := 0
	running := 0
	cancelled := 0

	// Count actual states from preflight items
	for _, item := range m.preflight.items {
		switch item.Run {
		case RunDone:
			completed++
		case RunRunning:
			running++
		case RunCancelled:
			cancelled++
		}
	}

	// Progress bar
	progressBar := m.renderProgressBar(completed, cancelled, total)
	content.WriteString(progressBar)
	content.WriteString("\n\n")

	// Show items with their current states. Use available viewport height.
	// Reserve 2 lines for progress + spacing and 1 line for optional summary.
	reservedLines := 3
	maxDisplay := m.viewport.Height - reservedLines
	if maxDisplay < 5 {
		maxDisplay = 5
	}

	startIdx := 0
	if len(m.preflight.items) > maxDisplay {
		// Show items around the current sync position or next pending if paused
		anchor := m.syncIndex
		if !m.syncing {
			// When paused, center on the first pending item for resuming visibility
			for i, it := range m.preflight.items {
				if it.Run == RunPending {
					anchor = i
					break
				}
			}
		}
		startIdx = anchor - maxDisplay/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx+maxDisplay > len(m.preflight.items) {
			startIdx = len(m.preflight.items) - maxDisplay
		}
	}

	endIdx := startIdx + maxDisplay
	if endIdx > len(m.preflight.items) {
		endIdx = len(m.preflight.items)
	}

	for i := startIdx; i < endIdx; i++ {
		item := m.preflight.items[i]
		status, color := m.getItemStatusDisplay(item.Run)

		// Show sync action and current state
		actionText := stateLabel(item.State)
		if item.Run == RunRunning {
			actionText += " (läuft...)"
		}

		// Format: [status] Name | slug (action) [issue]
		line := fmt.Sprintf("%s %s | %s (%s)",
			color.Render(status),
			item.Story.Name,
			subtleStyle.Render(item.Story.FullSlug),
			subtleStyle.Render(actionText))
		if item.Issue != "" {
			line += " " + warnStyle.Render("["+item.Issue+"]")
		}
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Show summary if we're not displaying all items
	if len(m.preflight.items) > (endIdx - startIdx) {
		content.WriteString("\n")
		summaryText := fmt.Sprintf("... zeige %d-%d von %d Items",
			startIdx+1, endIdx, len(m.preflight.items))
		content.WriteString(subtleStyle.Render(summaryText))
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
}

func (m Model) getItemStatusDisplay(runState string) (string, lipgloss.Style) {
	switch runState {
	case RunDone:
		return "✓", okStyle
	case RunRunning:
		return "◯", warnStyle
	case RunCancelled:
		return "✗", errorStyle
	default: // RunPending
		return "◦", subtleStyle
	}
}

func (m Model) renderProgressBar(completed, cancelled, total int) string {
	if total == 0 {
		return "Kein Fortschritt verfügbar"
	}

	processed := completed + cancelled
	percentage := float64(processed) / float64(total) * 100
	barWidth := 50
	completedWidth := int(float64(barWidth) * float64(completed) / float64(total))
	cancelledWidth := int(float64(barWidth) * float64(cancelled) / float64(total))

	var bar strings.Builder
	bar.WriteString("[")

	// Completed portion (green)
	for i := 0; i < completedWidth; i++ {
		bar.WriteString(okStyle.Render("█"))
	}

	// Cancelled portion (red)
	for i := 0; i < cancelledWidth; i++ {
		bar.WriteString(errorStyle.Render("█"))
	}

	// Empty portion
	remaining := barWidth - completedWidth - cancelledWidth
	for i := 0; i < remaining; i++ {
		bar.WriteString("░")
	}

	bar.WriteString("]")

	// Status text
	statusParts := []string{fmt.Sprintf("%d✓", completed)}
	if cancelled > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d✗", cancelled))
	}
	statusText := strings.Join(statusParts, " ")

	progressText := fmt.Sprintf("%s %.1f%% (%s/%d)",
		bar.String(), percentage, statusText, total)

	return focusStyle.Render(progressText)
}
