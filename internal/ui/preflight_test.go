package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

func TestStartPreflightDetectsCollisions(t *testing.T) {
	st1 := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	st2 := sb.Story{ID: 2, Name: "two", Slug: "two", FullSlug: "two"}
	tgt := sb.Story{ID: 3, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st1, st2}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st1.FullSlug] = true

	m.startPreflight()
	if m.state != statePreflight {
		t.Fatalf("expected statePreflight, got %v", m.state)
	}
	if len(m.preflight.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(m.preflight.items))
	}
	if !m.preflight.items[0].Collision {
		t.Fatalf("expected collision for first item")
	}
	if m.preflight.items[0].State != StateUpdate {
		t.Fatalf("expected state update, got %v", m.preflight.items[0].State)
	}
}

func TestPreflightMarksUnselectedFoldersSkipped(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "child", Slug: "child", FullSlug: "app/child", FolderID: &parent.ID}
	tgt := sb.Story{ID: 3, Name: "app", Slug: "app", FullSlug: "app"}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[child.FullSlug] = true

	m.startPreflight()

	if len(m.preflight.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.preflight.items))
	}
	var folderItem *PreflightItem
	for i := range m.preflight.items {
		if m.preflight.items[i].Story.ID == parent.ID {
			folderItem = &m.preflight.items[i]
		}
	}
	if folderItem == nil {
		t.Fatalf("folder not found in preflight items")
	}
	if !folderItem.Collision {
		t.Fatalf("expected collision for unselected folder")
	}
	if !folderItem.Skip {
		t.Fatalf("expected unselected folder to be skipped")
	}
	if folderItem.State != StateSkip {
		t.Fatalf("expected state skip for unselected folder, got %v", folderItem.State)
	}
}

func TestPreflightSkipToggleAndGlobal(t *testing.T) {
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	tgt := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st.FullSlug] = true
	m.startPreflight()

	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.preflight.items[0].Skip {
		t.Fatalf("expected item skipped after x")
	}
	if m.preflight.items[0].State != StateSkip {
		t.Fatalf("expected state skip after x, got %v", m.preflight.items[0].State)
	}
	m.preflight.items[0].Skip = false
	m.preflight.items[0].State = StateUpdate
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if !m.preflight.items[0].Skip {
		t.Fatalf("expected item skipped after X")
	}
	if m.preflight.items[0].State != StateSkip {
		t.Fatalf("expected state skip after X, got %v", m.preflight.items[0].State)
	}
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.selection.selected[st.FullSlug] {
		t.Fatalf("expected selection removed after c")
	}
	if len(m.preflight.items) != 0 {
		t.Fatalf("expected preflight list cleared after c, got %d", len(m.preflight.items))
	}
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.state != stateBrowseList {
		t.Fatalf("expected return to browse list on q")
	}
}

func TestDisplayPreflightItemDimsSlug(t *testing.T) {
	it := PreflightItem{Story: sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}}
	s := displayPreflightItem(it)
	expected := fmt.Sprintf("%s one  (one)", symbolStory)
	if s != expected {
		t.Fatalf("unexpected render for selected item: %q", s)
	}
	it.Selected = false
	s = displayPreflightItem(it)
	expected = fmt.Sprintf("%s %s  %s", symbolStory, subtleStyle.Render("one"), subtleStyle.Render("(one)"))
	if s != expected {
		t.Fatalf("expected dimmed slug and name when unselected: %q", s)
	}
	it.Selected = true
	it.Skip = true
	s = displayPreflightItem(it)
	if s != expected {
		t.Fatalf("expected dimmed slug and name when skipped: %q", s)
	}
}

func TestViewPreflightShowsStateCell(t *testing.T) {
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st.FullSlug] = true
	m.startPreflight()

	// Use the new viewport-based render
	m.updatePreflightViewport()
	out := m.renderPreflightHeader() + "\n" + m.renderViewportContent() + "\n" + m.renderPreflightFooter()
	if !strings.Contains(out, stateStyles[StateCreate].Render(stateLabel(StateCreate))) {
		t.Fatalf("expected create state cell")
	}

	m.preflight.items[0].Skip = true
	recalcState(&m.preflight.items[0])
	m.updatePreflightViewport()
	out = m.renderPreflightHeader() + "\n" + m.renderViewportContent() + "\n" + m.renderPreflightFooter()
	if !strings.Contains(out, stateStyles[StateSkip].Render(stateLabel(StateSkip))) {
		t.Fatalf("expected skip state cell")
	}
}

func TestOptimizePreflightDedupesFolders(t *testing.T) {
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	m := InitialModel()
	m.preflight.items = []PreflightItem{
		{Story: folder, Selected: true},
		{Story: folder, Selected: true},
	}
	m.optimizePreflight()
	if len(m.preflight.items) != 1 {
		t.Fatalf("expected 1 item after dedupe, got %d", len(m.preflight.items))
	}
}

func TestOptimizePreflight_FullFolderSelection(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "app/one", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "two", Slug: "two", FullSlug: "app/two", FolderID: &parent.ID}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child1.FullSlug] = true
	m.selection.selected[child2.FullSlug] = true
	m.startPreflight()
	m.optimizePreflight()
	if len(m.preflight.items) != 3 {
		t.Fatalf("expected 3 items after optimization, got %d", len(m.preflight.items))
	}
}
