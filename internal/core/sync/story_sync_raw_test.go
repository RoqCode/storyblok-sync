package sync

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"storyblok-sync/internal/sb"
)

// mockStoryRawSyncAPI implements SyncAPI + storyRawAPI for raw-path tests
type mockStoryRawSyncAPI struct {
	// minimal stores
	sourceTypedByID map[int]sb.Story
	sourceRawByID   map[int]map[string]interface{}
	targetBySlug    map[string]sb.Story

	// call tracking
	rawCreates   []map[string]interface{}
	rawUpdates   []map[string]interface{}
	typedCreates []sb.Story
	typedUpdates []sb.Story

	// publish tracking
	publishFlags []bool
}

func newMockStoryRawSyncAPI() *mockStoryRawSyncAPI {
	return &mockStoryRawSyncAPI{
		sourceTypedByID: make(map[int]sb.Story),
		sourceRawByID:   make(map[int]map[string]interface{}),
		targetBySlug:    make(map[string]sb.Story),
	}
}

// ---- SyncAPI (typed) ----
func (m *mockStoryRawSyncAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if st, ok := m.targetBySlug[slug]; ok {
		return []sb.Story{st}, nil
	}
	return []sb.Story{}, nil
}

func (m *mockStoryRawSyncAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	return m.sourceTypedByID[storyID], nil
}

func (m *mockStoryRawSyncAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.typedCreates = append(m.typedCreates, st)
	st.ID = 100 + len(m.typedCreates)
	m.targetBySlug[st.FullSlug] = st
	m.publishFlags = append(m.publishFlags, publish)
	return st, nil
}

func (m *mockStoryRawSyncAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.typedUpdates = append(m.typedUpdates, st)
	m.targetBySlug[st.FullSlug] = st
	m.publishFlags = append(m.publishFlags, publish)
	return st, nil
}

func (m *mockStoryRawSyncAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	return nil
}

// ---- storyRawAPI ----
func (m *mockStoryRawSyncAPI) GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	return m.sourceRawByID[storyID], nil
}

func (m *mockStoryRawSyncAPI) CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.rawCreates = append(m.rawCreates, story)
	st := sb.Story{ID: 100 + len(m.rawCreates), FullSlug: asStringTest(story["full_slug"])}
	m.targetBySlug[st.FullSlug] = st
	m.publishFlags = append(m.publishFlags, publish)
	return st, nil
}

func (m *mockStoryRawSyncAPI) UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.rawUpdates = append(m.rawUpdates, story)
	slug := asStringTest(story["full_slug"])
	st := m.targetBySlug[slug]
	if st.ID == 0 {
		st = sb.Story{ID: storyID, FullSlug: slug}
	}
	m.targetBySlug[slug] = st
	m.publishFlags = append(m.publishFlags, publish)
	return st, nil
}

// helper used by orchestrator shim; duplicated to keep test self-contained
func asStringTest(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func TestSyncStory_Create_UsesRawPayload(t *testing.T) {
	api := newMockStoryRawSyncAPI()
	// Source story typed + raw
	sourceID := 1
	api.sourceTypedByID[sourceID] = sb.Story{ID: sourceID, Slug: "page", FullSlug: "folder/page", Content: json.RawMessage(`{"component":"page","meta":{"x":1}}`), UUID: "uuid-s"}
	raw := map[string]interface{}{
		"uuid":      "uuid-s",
		"name":      "Page",
		"slug":      "page",
		"full_slug": "folder/page",
		"content":   map[string]interface{}{"component": "page", "meta": map[string]interface{}{"x": 1}},
		"is_folder": false,
	}
	api.sourceRawByID[sourceID] = raw

	syncer := NewStorySyncer(api, 10, 20)
	st := sb.Story{ID: sourceID, Slug: "page", FullSlug: "folder/page"}
	ctx := context.Background()
	if _, err := syncer.SyncStory(ctx, st, true); err != nil {
		t.Fatalf("sync create failed: %v", err)
	}

	if len(api.rawCreates) != 1 {
		t.Fatalf("expected raw create, got %d", len(api.rawCreates))
	}
	if len(api.typedCreates) != 0 {
		t.Fatalf("unexpected typed create")
	}
	// Parent set default 0 when none
	if pid, ok := api.rawCreates[0]["parent_id"].(int); !ok || pid != 0 {
		t.Errorf("expected parent_id 0, got %v", api.rawCreates[0]["parent_id"])
	}
	// Raw meta preserved
	if !reflect.DeepEqual(api.rawCreates[0]["content"], raw["content"]) {
		t.Errorf("content mismatch: %+v vs %+v", api.rawCreates[0]["content"], raw["content"])
	}
}

func TestSyncStory_Update_UsesRawPayload(t *testing.T) {
	api := newMockStoryRawSyncAPI()
	// Existing in target
	existing := sb.Story{ID: 200, FullSlug: "folder/page"}
	api.targetBySlug[existing.FullSlug] = existing
	// Source typed + raw
	sourceID := 2
	api.sourceTypedByID[sourceID] = sb.Story{ID: sourceID, Slug: "page", FullSlug: "folder/page", Content: json.RawMessage(`{"component":"page","meta":{"k":"v"}}`), UUID: "uuid-2"}
	raw := map[string]interface{}{
		"uuid":       "uuid-2",
		"name":       "Page",
		"slug":       "page",
		"full_slug":  "folder/page",
		"is_folder":  false,
		"content":    map[string]interface{}{"component": "page", "meta": map[string]interface{}{"k": "v"}},
		"custom_top": map[string]interface{}{"keep": true},
	}
	api.sourceRawByID[sourceID] = raw

	syncer := NewStorySyncer(api, 10, 20)
	st := sb.Story{ID: sourceID, Slug: "page", FullSlug: existing.FullSlug, UUID: "uuid-2"}
	ctx := context.Background()
	if _, err := syncer.SyncStory(ctx, st, true); err != nil {
		t.Fatalf("sync update failed: %v", err)
	}

	if len(api.rawUpdates) != 1 {
		t.Fatalf("expected raw update, got %d", len(api.rawUpdates))
	}
	if len(api.typedUpdates) != 0 {
		t.Fatalf("unexpected typed update")
	}
	// Unknown top-level preserved
	if _, ok := api.rawUpdates[0]["custom_top"]; !ok {
		t.Errorf("expected custom_top to be preserved in raw update")
	}
}

func TestSyncFolder_Create_UsesRawPayload(t *testing.T) {
	api := newMockStoryRawSyncAPI()
	// Source folder typed + raw
	sourceID := 3
	api.sourceTypedByID[sourceID] = sb.Story{ID: sourceID, Slug: "folder", FullSlug: "folder", IsFolder: true, Content: json.RawMessage(`{}`)}
	raw := map[string]interface{}{
		"uuid":           "uuid-f",
		"name":           "Folder",
		"slug":           "folder",
		"full_slug":      "folder",
		"is_folder":      true,
		"folder_setting": map[string]interface{}{"foo": "bar"},
	}
	api.sourceRawByID[sourceID] = raw

	syncer := NewStorySyncer(api, 10, 20)
	folder := sb.Story{ID: sourceID, Slug: "folder", FullSlug: "folder", IsFolder: true}
	ctx := context.Background()
	if _, err := syncer.SyncFolder(ctx, folder, false); err != nil {
		t.Fatalf("sync folder create failed: %v", err)
	}

	if len(api.rawCreates) != 1 {
		t.Fatalf("expected raw folder create, got %d", len(api.rawCreates))
	}
	if _, ok := api.rawCreates[0]["folder_setting"]; !ok {
		t.Errorf("expected folder_setting to be preserved in raw create")
	}
}
