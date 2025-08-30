package sync

import (
	"context"
	"encoding/json"
	"testing"

	"storyblok-sync/internal/sb"
)

// loggingAPI implements both SyncAPI and FolderAPI behaviors with rich logging
// It simulates a source and target space and records all fetched and pushed data.
type loggingAPI struct {
	// Space IDs
	sourceSpaceID int
	targetSpaceID int

	// Test handle for logging
	t *testing.T

	// Source data
	sourceBySlug  map[string]sb.Story            // minimal typed index for source (e.g., folders)
	sourceContent map[int]sb.Story               // typed stories with full content by ID (GetStoryWithContent)
	sourceRawByID map[int]map[string]interface{} // raw stories by ID (GetStoryRaw)

	// Target data (simulates remote storage)
	targetBySlug map[string]sb.Story // created/updated items, indexed by slug
	nextID       int                 // auto-increment ID for creations

	// Logs
	logFetchRaw   []map[string]interface{}
	logFetchTyped []sb.Story
	logPushRaw    []map[string]interface{}
	logPushTyped  []sb.Story
}

func newLoggingAPI(t *testing.T, sourceID, targetID int) *loggingAPI {
	return &loggingAPI{
		sourceSpaceID: sourceID,
		targetSpaceID: targetID,
		t:             t,
		sourceBySlug:  make(map[string]sb.Story),
		sourceContent: make(map[int]sb.Story),
		sourceRawByID: make(map[int]map[string]interface{}),
		targetBySlug:  make(map[string]sb.Story),
		nextID:        100,
	}
}

// mockFolderReport implements Report interface for EnsureFolderPathStatic tests
type mockFolderReport struct{}

func (*mockFolderReport) AddSuccess(slug, operation string, duration int64, story *sb.Story) {}

// ---- Helpers ----
func (api *loggingAPI) logJSON(prefix string, v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	api.t.Logf("%s: %s", prefix, string(b))
}

// ---- SyncAPI methods ----
func (api *loggingAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if spaceID == api.sourceSpaceID {
		if st, ok := api.sourceBySlug[slug]; ok {
			return []sb.Story{st}, nil
		}
		return []sb.Story{}, nil
	}
	// target space
	if st, ok := api.targetBySlug[slug]; ok {
		return []sb.Story{st}, nil
	}
	return []sb.Story{}, nil
}

func (api *loggingAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	st := api.sourceContent[storyID]
	api.logFetchTyped = append(api.logFetchTyped, st)
	api.logJSON("SOURCE_TYPED", st)
	return st, nil
}

// typed methods no longer used

func (api *loggingAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	// Update in target map by finding story with ID
	for slug, st := range api.targetBySlug {
		if st.ID == storyID {
			st.UUID = uuid
			api.targetBySlug[slug] = st
			break
		}
	}
	return nil
}

// ---- Raw methods ----
func (api *loggingAPI) GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	raw := api.sourceRawByID[storyID]
	api.logFetchRaw = append(api.logFetchRaw, raw)
	api.logJSON("SOURCE_RAW", raw)
	return raw, nil
}

func (api *loggingAPI) CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	// Simulate target create from raw
	res := sb.Story{
		ID:       api.nextID,
		IsFolder: true,
	}
	api.nextID++
	if v, ok := story["full_slug"].(string); ok {
		res.FullSlug = v
		api.targetBySlug[v] = res
	}
	if pid, ok := story["parent_id"].(int); ok {
		res.FolderID = &pid
	}
	api.logPushRaw = append(api.logPushRaw, story)
	api.logJSON("PUSH_RAW", story)
	return res, nil
}

func (api *loggingAPI) UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	// Simulate target update from raw
	slug, _ := story["full_slug"].(string)
	st := api.targetBySlug[slug]
	if st.ID == 0 {
		st = sb.Story{ID: storyID, FullSlug: slug}
	}
	api.targetBySlug[slug] = st
	api.logPushRaw = append(api.logPushRaw, story)
	api.logJSON("PUSH_RAW_UPDATE", story)
	return st, nil
}

// ---- Test ----
func TestDataFlowLogging_TwoFoldersTwoStories(t *testing.T) {
	// Setup spaces and API
	sourceID := 1
	targetID := 2
	api := newLoggingAPI(t, sourceID, targetID)

	// Source folders (raw payloads)
	rawFolderA := map[string]interface{}{
		"name":           "A",
		"slug":           "a",
		"full_slug":      "a",
		"is_folder":      true,
		"uuid":           "uuid-folder-a",
		"content":        map[string]interface{}{"content_types": []string{"article"}},
		"folder_setting": map[string]interface{}{"foo": "bar"},
	}
	rawFolderB := map[string]interface{}{
		"name":      "B",
		"slug":      "b",
		"full_slug": "b",
		"is_folder": true,
		"uuid":      "uuid-folder-b",
		"content":   map[string]interface{}{"content_types": []string{"article"}},
	}

	// Index folders in source: GetStoriesBySlug (source) returns typed minimal folder with ID
	api.sourceBySlug["a"] = sb.Story{ID: 10, FullSlug: "a", Slug: "a", IsFolder: true}
	api.sourceBySlug["b"] = sb.Story{ID: 11, FullSlug: "b", Slug: "b", IsFolder: true}
	api.sourceRawByID[10] = rawFolderA
	api.sourceRawByID[11] = rawFolderB

	// Source stories with full typed content and raw
	story1 := sb.Story{
		ID:       20,
		Slug:     "page1",
		FullSlug: "a/page1",
		Content:  mustJSON(map[string]interface{}{"component": "article", "meta": map[string]interface{}{"x": 1}, "extra_field": map[string]interface{}{"y": "z"}}),
		UUID:     "uuid-story-page1",
	}
	story2 := sb.Story{
		ID:       21,
		Slug:     "page2",
		FullSlug: "b/page2",
		Content:  mustJSON(map[string]interface{}{"component": "article", "meta": map[string]interface{}{"k": "v"}}),
		UUID:     "uuid-story-page2",
	}
	api.sourceContent[20] = story1
	api.sourceContent[21] = story2
	api.sourceRawByID[20] = map[string]interface{}{
		"uuid": "uuid-story-page1", "name": "Page1", "slug": "page1", "full_slug": "a/page1",
		"content":   map[string]interface{}{"component": "article", "meta": map[string]interface{}{"x": 1}, "extra_field": map[string]interface{}{"y": "z"}},
		"is_folder": false,
	}
	api.sourceRawByID[21] = map[string]interface{}{
		"uuid": "uuid-story-page2", "name": "Page2", "slug": "page2", "full_slug": "b/page2",
		"content":   map[string]interface{}{"component": "article", "meta": map[string]interface{}{"k": "v"}},
		"is_folder": false,
	}

	// 1) Ensure full folder paths for both stories
	report := &mockFolderReport{}
	_, err := EnsureFolderPathStatic(api, report, []sb.Story{}, sourceID, targetID, story1.FullSlug, false)
	if err != nil {
		t.Fatalf("ensure path for story1: %v", err)
	}
	_, err = EnsureFolderPathStatic(api, report, []sb.Story{}, sourceID, targetID, story2.FullSlug, false)
	if err != nil {
		t.Fatalf("ensure path for story2: %v", err)
	}

	// 2) Sync both stories and record all pushes
	syncer := NewStorySyncer(api, sourceID, targetID)
	ctx := context.Background()
	if _, err := syncer.SyncStory(ctx, story1, false); err != nil {
		t.Fatalf("sync story1: %v", err)
	}
	if _, err := syncer.SyncStory(ctx, story2, false); err != nil {
		t.Fatalf("sync story2: %v", err)
	}

	// 3) Summary logs to make comparison easier in output
	t.Log("==== SUMMARY: SOURCE RAW FETCHES ====")
	for i, r := range api.logFetchRaw {
		api.logJSON(fnLabel("SRC_RAW", i), r)
	}
	t.Log("==== SUMMARY: SOURCE TYPED FETCHES ====")
	for i, st := range api.logFetchTyped {
		api.logJSON(fnLabel("SRC_TYPED", i), st)
	}
	t.Log("==== SUMMARY: PUSH RAW (folders) ====")
	for i, r := range api.logPushRaw {
		api.logJSON(fnLabel("PUSH_RAW", i), r)
	}
	t.Log("==== SUMMARY: PUSH TYPED (stories) ====")
	for i, st := range api.logPushTyped {
		api.logJSON(fnLabel("PUSH_TYPED", i), st)
	}
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

func fnLabel(prefix string, i int) string {
	return prefix + "_" + jsonIndex(i)
}

func jsonIndex(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
