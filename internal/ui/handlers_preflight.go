package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"storyblok-sync/internal/sb"
)

func (m Model) handlePreflightKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.syncing {
		return m, nil
	}
	key := msg.String()
	switch key {
	case "l":
		// Expand folder under cursor
		if len(m.preflight.items) > 0 && m.preflight.listIndex < len(m.preflight.items) {
			// Find actual item index from visible order
			if m.preflight.listIndex >= 0 && m.preflight.listIndex < len(m.preflight.visibleIdx) {
				idx := m.preflight.visibleIdx[m.preflight.listIndex]
				st := m.preflight.items[idx].Story
				if st.IsFolder {
					m.folderCollapsed[st.ID] = false
					m.refreshPreflightVisible()
					m.updateViewportContent()
				}
			}
		}
	case "h":
		// Collapse folder or parent
		if len(m.preflight.items) > 0 && m.preflight.listIndex < len(m.preflight.items) {
			if m.preflight.listIndex >= 0 && m.preflight.listIndex < len(m.preflight.visibleIdx) {
				idx := m.preflight.visibleIdx[m.preflight.listIndex]
				st := m.preflight.items[idx].Story
				if st.IsFolder {
					m.folderCollapsed[st.ID] = true
					m.refreshPreflightVisible()
					m.updateViewportContent()
				} else if st.FolderID != nil {
					pid := *st.FolderID
					m.folderCollapsed[pid] = true
					m.refreshPreflightVisible()
					// Move cursor to parent if visible
					for vis, ii := range m.preflight.visibleIdx {
						if m.preflight.items[ii].Story.ID == pid {
							m.preflight.listIndex = vis
							break
						}
					}
					m.ensurePreflightCursorVisible()
					m.updateViewportContent()
				}
			}
		}
	case "H":
		// Collapse all folders
		for id := range m.folderCollapsed {
			m.folderCollapsed[id] = true
		}
		m.refreshPreflightVisible()
		m.updateViewportContent()
	case "L":
		// Expand all folders
		for id := range m.folderCollapsed {
			m.folderCollapsed[id] = false
		}
		m.refreshPreflightVisible()
		m.updateViewportContent()
	case "j", "down":
		// Use visible list length
		maxIdx := len(m.preflight.visibleIdx)
		if maxIdx == 0 {
			maxIdx = len(m.preflight.items)
		}
		if m.preflight.listIndex < maxIdx-1 {
			m.preflight.listIndex++
			m.ensurePreflightCursorVisible()
			m.updateViewportContent()
		}
	case "k", "up":
		if m.preflight.listIndex > 0 {
			m.preflight.listIndex--
			m.ensurePreflightCursorVisible()
			m.updateViewportContent()
		}
	case "ctrl+d", "pgdown":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		maxIdx := len(m.preflight.visibleIdx)
		if maxIdx == 0 {
			maxIdx = len(m.preflight.items)
		}
		if m.preflight.listIndex+jump < maxIdx {
			m.preflight.listIndex += jump
		} else {
			m.preflight.listIndex = maxIdx - 1
		}
		m.ensurePreflightCursorVisible()
		m.updateViewportContent()
	case "ctrl+u", "pgup":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		if m.preflight.listIndex-jump >= 0 {
			m.preflight.listIndex -= jump
		} else {
			m.preflight.listIndex = 0
		}
		m.ensurePreflightCursorVisible()
		m.updateViewportContent()
	case "x":
		if len(m.preflight.items) > 0 {
			it := &m.preflight.items[m.preflight.listIndex]
			if it.Collision && it.Selected {
				it.Skip = !it.Skip
				recalcState(it)
				m.updateViewportContent()
			}
		}
	case "X":
		for i := range m.preflight.items {
			if m.preflight.items[i].Collision && m.preflight.items[i].Selected {
				m.preflight.items[i].Skip = true
				recalcState(&m.preflight.items[i])
			}
		}
		m.updateViewportContent()
	case "c":
		removed := false
		for _, it := range m.preflight.items {
			if it.Skip {
				delete(m.selection.selected, it.Story.FullSlug)
				removed = true
			}
		}
		if removed {
			m.startPreflight()
		}
	case "esc", "q":
		// restore browse collapse state
		if m.collapsedBeforePreflight != nil {
			for id := range m.folderCollapsed {
				if prev, ok := m.collapsedBeforePreflight[id]; ok {
					m.folderCollapsed[id] = prev
				}
			}
			m.collapsedBeforePreflight = nil
		}
		m.refreshVisible()
		m.state = stateBrowseList
		m.updateViewportContent()
		return m, nil
	case "enter":
		m.optimizePreflight()
		if len(m.preflight.items) == 0 {
			m.statusMsg = "Keine Items zum Sync"
			return m, nil
		}
		m.plan = SyncPlan{Items: append([]PreflightItem(nil), m.preflight.items...)}
		m.syncing = true
		m.syncIndex = 0
		m.api = sb.New(m.cfg.Token)
		m.state = stateSync

		// Set up cancellation context for sync operations
		m.syncContext, m.syncCancel = context.WithCancel(context.Background())

		// Initialize comprehensive report with space information
		sourceSpaceName := ""
		targetSpaceName := ""
		if m.sourceSpace != nil {
			sourceSpaceName = fmt.Sprintf("%s (%d)", m.sourceSpace.Name, m.sourceSpace.ID)
		}
		if m.targetSpace != nil {
			targetSpaceName = fmt.Sprintf("%s (%d)", m.targetSpace.Name, m.targetSpace.ID)
		}
		m.report = *NewReport(sourceSpaceName, targetSpaceName)

		m.statusMsg = fmt.Sprintf("Synchronisiere %d Items…", len(m.preflight.items))
		return m, tea.Batch(m.spinner.Tick, m.runNextItem())
	}
	return m, nil
}

func (m *Model) startPreflight() {
	target := make(map[string]bool, len(m.storiesTarget))
	for _, st := range m.storiesTarget {
		target[st.FullSlug] = true
	}
	included := make(map[int]bool)
	for slug, v := range m.selection.selected {
		if !v {
			continue
		}
		if idx := m.indexBySlug(slug); idx >= 0 {
			m.includeAncestors(idx, included)
		}
	}
	if len(included) == 0 {
		m.preflight = PreflightState{}
		m.statusMsg = "Keine Stories markiert."
		return
	}
	children := make(map[int][]int)
	roots := make([]int, 0)
	for i, st := range m.storiesSource {
		if !included[i] {
			continue
		}
		if st.FolderID != nil {
			if pIdx, ok := m.storyIdx[*st.FolderID]; ok && included[pIdx] {
				children[pIdx] = append(children[pIdx], i)
				continue
			}
		}
		roots = append(roots, i)
	}
	items := make([]PreflightItem, 0, len(included))
	var walk func(int)
	walk = func(idx int) {
		st := m.storiesSource[idx]
		sel := m.selection.selected[st.FullSlug]
		it := PreflightItem{Story: st, Collision: target[st.FullSlug], Selected: sel, Skip: !sel}
		recalcState(&it)
		items = append(items, it)
		for _, ch := range children[idx] {
			walk(ch)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	// Initialize preflight state; preflight view should start fully expanded.
	// We'll build visibleIdx like browse does, using current folderCollapsed but ensure all expanded at start.
	// Save previous collapse state to restore later if needed.
	// preserve previous collapse state and start preflight fully expanded
	m.collapsedBeforePreflight = make(map[int]bool, len(m.folderCollapsed))
	for id, v := range m.folderCollapsed {
		m.collapsedBeforePreflight[id] = v
	}
	for id := range m.folderCollapsed {
		m.folderCollapsed[id] = false
	}

	m.preflight = PreflightState{items: items, listIndex: 0}
	m.refreshPreflightVisible()
	m.state = statePreflight
	collisions := 0
	for _, it := range items {
		if it.Collision {
			collisions++
		}
	}
	m.statusMsg = fmt.Sprintf("Preflight: %d Items, %d Kollisionen", len(items), collisions)
	m.updateViewportContent()
}

func (m *Model) ensurePreflightCursorVisible() {
	n := len(m.preflight.items)
	if n == 0 {
		m.preflight.listIndex = 0
		return
	}
	if m.preflight.listIndex < 0 {
		m.preflight.listIndex = 0
	}
	if m.preflight.listIndex > n-1 {
		m.preflight.listIndex = n - 1
	}
	// Calculate which line in the viewport content the cursor is on
	cursorLine := m.calculatePreflightCursorLine()

	// Adjust viewport using shared helper
	m.ensureCursorInViewport(cursorLine)
}

func (m *Model) calculatePreflightCursorLine() int {
	// Match renderPreflightContent exactly, including wrapping
	total := len(m.preflight.items)
	if total == 0 || m.preflight.listIndex <= 0 {
		return 0
	}

	// Visible order and stories via shared helper
	stories, order := m.visibleOrderPreflight()
	treeLines := generateTreeLinesFromStories(stories)

	// Width budget equals renderPreflightContent (cursorCell + stateCell + content)
	contentWidth := m.width - 4
	if contentWidth <= 0 {
		contentWidth = 80
	}

	// Sum wrapped lines up to the cursor's visible position (exclusive)
	sum := 0
	max := m.preflight.listIndex
	if max > len(order) {
		max = len(order)
	}
	for visPos := 0; visPos < max; visPos++ {
		if visPos >= len(treeLines) {
			break
		}
		idx := order[visPos]
		it := m.preflight.items[idx]

		// Recreate exact content prefix (collision sign/padding)
		content := treeLines[visPos]
		if it.Collision {
			content = collisionSign + " " + content
		} else {
			content = "  " + content
		}

		// Apply same styling (without cursor highlight)
		lineStyle := lipgloss.NewStyle().Width(contentWidth)
		if strings.ToLower(it.State) == StateSkip {
			lineStyle = lineStyle.Faint(true)
		}
		styled := lineStyle.Render(content)
		sum += m.countWrappedLines(styled)
	}
	return sum
}

// refreshPreflightVisible builds visibleIdx for preflight items based on folderCollapsed
func (m *Model) refreshPreflightVisible() {
	// Construct children map using storiesSource relationships for consistent IDs
	children := make(map[int][]int)
	for pi, it := range m.preflight.items {
		st := it.Story
		if st.FolderID != nil {
			children[*st.FolderID] = append(children[*st.FolderID], pi)
		}
	}

	m.preflight.visibleIdx = m.preflight.visibleIdx[:0]

	var addVisible func(int)
	addVisible = func(idx int) {
		st := m.preflight.items[idx].Story
		m.preflight.visibleIdx = append(m.preflight.visibleIdx, idx)
		if st.IsFolder && !m.folderCollapsed[st.ID] {
			for _, ch := range children[st.ID] {
				addVisible(ch)
			}
		}
	}

	for i, it := range m.preflight.items {
		if it.Story.FolderID == nil {
			addVisible(i)
		}
	}
}
