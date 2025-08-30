package ui

import (
	"context"
	"storyblok-sync/internal/config"
	sync "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/sb"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// --- Model / State ---
type state int

const (
	stateWelcome state = iota
	stateTokenPrompt
	stateValidating
	stateSpaceSelect
	stateScanning
	stateBrowseList
	statePreflight
	stateSync
	stateReport
	stateQuit
)

type SelectionState struct {
	// browse list (source)
	listIndex int
	selected  map[string]bool // key: FullSlug (oder Full Path)
}

type FilterState struct {
	// prefix-filter
	prefixing   bool
	prefixInput textinput.Model
	prefix      string // z.B. "a__portal/de"
}

type SearchState struct {
	// searching
	searching   bool
	searchInput textinput.Model
	query       string // aktueller Suchstring
	filteredIdx []int  // Mapping: sichtbarer Index -> original Index
}

// Unified Preflight state with core
// State values: "create", "update", "skip"
// Run values:   "pending", "running", "success", "failed"
const (
	StateCreate = "create"
	StateUpdate = "update"
	StateSkip   = "skip"

	RunPending   = "pending"
	RunRunning   = "running"
	RunDone      = "success"
	RunCancelled = "failed"
)

// Use core PreflightItem directly (includes optional Issue field)
type PreflightItem = sync.PreflightItem

// recalcState updates the textual state based on Skip/Collision
func recalcState(it *PreflightItem) {
	switch {
	case it.Skip:
		it.State = StateSkip
	case it.Collision:
		it.State = StateUpdate
	default:
		it.State = StateCreate
	}
}

type PreflightState struct {
	items     []PreflightItem
	listIndex int
	// visibleIdx maps visible list positions to indices in items
	// to support folder collapse/expand like in browse view
	visibleIdx []int
}

type SyncPlan struct {
	Items []PreflightItem
}

type Model struct {
	state         state
	cfg           config.Config
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	validateErr   error
	width, height int

	// viewport for scrollable content
	viewport viewport.Model

	// spinner for loading states
	spinner     spinner.Model
	syncing     bool
	syncIndex   int
	syncCancel  context.CancelFunc // for cancelling sync operations
	syncContext context.Context    // cancellable context for sync
	api         *sb.Client
	report      Report

	// token input
	ti textinput.Model

	// spaces & selection
	spaces          []sb.Space
	selectedIndex   int
	selectingSource bool
	sourceSpace     *sb.Space
	targetSpace     *sb.Space

	// scan results
	storiesSource []sb.Story
	storiesTarget []sb.Story

	// tree state
	storyIdx        map[int]int  // Story ID -> index in storiesSource
	folderCollapsed map[int]bool // Folder ID -> collapsed?
	visibleIdx      []int        // indices of visible storiesSource entries

	selection SelectionState
	filter    FilterState
	search    SearchState
	filterCfg FilterConfig // Konfiguration für Such- und Filterparameter

	preflight PreflightState
	plan      SyncPlan

	// preserve browse collapse state when entering preflight
	collapsedBeforePreflight map[int]bool
}
