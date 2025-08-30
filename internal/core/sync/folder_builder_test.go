package sync

import (
	"context"
	"testing"

	"storyblok-sync/internal/sb"
)

// Minimal mock implementing FolderAPI for folder builder tests
type mockFolderAPI struct {
	srcBySlug map[string][]sb.Story
	tgtBySlug map[string][]sb.Story
	raw       map[int]map[string]interface{}
	created   []map[string]interface{}
}

func (m *mockFolderAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if spaceID == 1 { // source
		return m.srcBySlug[slug], nil
	}
	return m.tgtBySlug[slug], nil
}
func (m *mockFolderAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	return sb.Story{}, nil
}
func (m *mockFolderAPI) GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	return m.raw[storyID], nil
}
func (m *mockFolderAPI) CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.created = append(m.created, story)
	id := len(m.created) + 100
	slug := ""
	if s, ok := story["full_slug"].(string); ok {
		slug = s
	}
	st := sb.Story{ID: id, FullSlug: slug, IsFolder: true}
	m.tgtBySlug[slug] = []sb.Story{st}
	return st, nil
}
func (m *mockFolderAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	return nil
}

func TestEnsureFolderPath_CreatesMissingParents(t *testing.T) {
	api := &mockFolderAPI{srcBySlug: map[string][]sb.Story{}, tgtBySlug: map[string][]sb.Story{}, raw: map[int]map[string]interface{}{}}
	// Source raw payloads for two segments
	api.raw[1] = map[string]interface{}{"uuid": "u1", "name": "app", "slug": "app", "full_slug": "app", "is_folder": true}
	api.raw[2] = map[string]interface{}{"uuid": "u2", "name": "de", "slug": "de", "full_slug": "app/de", "is_folder": true}

	// Pretend source lookup returns folder IDs 1,2 for slugs
	api.srcBySlug["app"] = []sb.Story{{ID: 1, IsFolder: true, FullSlug: "app"}}
	api.srcBySlug["app/de"] = []sb.Story{{ID: 2, IsFolder: true, FullSlug: "app/de"}}

	b := NewFolderPathBuilder(api, nil, nil, 1, 2, false)
	created, err := b.EnsureFolderPath("app/de/page")
	if err != nil {
		t.Fatalf("EnsureFolderPath failed: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 folders created, got %d", len(created))
	}
	// parent_id should be set; check stored payloads
	if got := api.created[0]["parent_id"]; got != 0 {
		t.Errorf("expected root parent_id 0, got %v", got)
	}
	if got := api.created[1]["parent_id"]; got == 0 {
		t.Errorf("expected non-root parent_id for second folder, got %v", got)
	}
}
