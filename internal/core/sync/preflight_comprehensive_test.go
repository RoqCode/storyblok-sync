package sync

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestPreflightPlanner_Optimize_AddsMissingFoldersAndSorts(t *testing.T) {
	src := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/de", IsFolder: true, FolderID: ptr(1)},
		{ID: 3, FullSlug: "app/de/page", IsFolder: false, FolderID: ptr(2)},
	}
	tgt := []sb.Story{}

	pp := NewPreflightPlanner(src, tgt)
	items := []PreflightItem{{Story: src[2], Selected: true, State: StateCreate}}
	out := pp.OptimizePreflight(items)
	if len(out) != 3 {
		t.Fatalf("expected 3 items incl. 2 folders, got %d", len(out))
	}
	if !out[0].Story.IsFolder || !out[1].Story.IsFolder || out[2].Story.IsFolder {
		t.Fatalf("expected folders first, then story")
	}
}

func ptr(i int) *int { return &i }
