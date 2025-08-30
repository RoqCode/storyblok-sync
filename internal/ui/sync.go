package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	sync "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/sb"
)

// Constants for sync operations and timeouts
const (
	// API timeout constants
	defaultTimeout = 15 * time.Second
	longTimeout    = 30 * time.Second

	// Operation types - use sync package constants
	operationCreate = sync.OperationCreate
	operationUpdate = sync.OperationUpdate
	operationSkip   = sync.OperationSkip
)

// Legacy wrapper for content management - now uses the extracted module
type contentManager struct {
	*sync.ContentManager
}

// newContentManager creates a new content manager using the extracted module
func newContentManager(api folderAPI, spaceID int) *contentManager {
	return &contentManager{
		ContentManager: sync.NewContentManager(api, spaceID),
	}
}

// ensureContent is a legacy wrapper that calls the new EnsureContent method
func (cm *contentManager) ensureContent(ctx context.Context, story sb.Story) (sb.Story, error) {
	return cm.EnsureContent(ctx, story)
}

// Legacy wrappers for utility functions - now uses the extracted module
func prepareStoryForCreation(story sb.Story) sb.Story { return sync.PrepareStoryForCreation(story) }

func prepareStoryForUpdate(source, target sb.Story) sb.Story {
	return sync.PrepareStoryForUpdate(source, target)
}

// resolveParentFolder resolves and sets the correct parent folder ID for a story
func (m *Model) resolveParentFolder(ctx context.Context, story sb.Story) (sb.Story, string, error) {
	var warning string

	if story.FolderID == nil {
		return story, warning, nil
	}

	parentSlugStr := sync.ParentSlug(story.FullSlug)

	if parentSlugStr == "" {
		story.FolderID = nil
		return story, warning, nil
	}

	targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlugStr)
	if err != nil {
		return story, warning, err
	}

	if len(targetParents) > 0 {
		story.FolderID = &targetParents[0].ID
	} else {
		story.FolderID = nil
		warning = "Parent folder not found in target space"
	}

	return story, warning, nil
}

// syncUUID updates the UUID of a target story if it differs from source
func (m *Model) syncUUID(ctx context.Context, targetStory sb.Story, sourceUUID string) error {
	if targetStory.UUID == sourceUUID || sourceUUID == "" {
		return nil
	}

	log.Printf("DEBUG: Updating UUID for %s from %s to %s",
		targetStory.FullSlug, targetStory.UUID, sourceUUID)

	err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, targetStory.ID, sourceUUID)
	if err != nil {
		log.Printf("Warning: failed to update UUID for story %s: %v", targetStory.FullSlug, err)
		return err
	}

	return nil
}

func ensureDefaultContent(story sb.Story) sb.Story {
	return sync.EnsureDefaultContent(story)
}

// Legacy type aliases for backward compatibility
type syncItemResult = sync.SyncItemResult
type syncResultMsg = sync.SyncResultMsg
type syncCancelledMsg = sync.SyncCancelledMsg

// Legacy wrapper for logging functions - now uses the extracted module
func logError(operation, slug string, err error, story *sb.Story) {
	sync.LogError(operation, slug, err, story)
}

func logWarning(operation, slug, warning string, story *sb.Story) {
	sync.LogWarning(operation, slug, warning, story)
}

func logSuccess(operation, slug string, duration int64, targetStory *sb.Story) {
	sync.LogSuccess(operation, slug, duration, targetStory)
}

// logExtendedErrorContext is now handled within the sync.LogError function

func getFolderPaths(slug string) []string {
	return sync.GetFolderPaths(slug)
}

// buildTargetFolderMap creates a map of existing folders in target space for quick lookup
func (m *Model) buildTargetFolderMap() map[string]sb.Story {
	planner := sync.NewPreflightPlanner(m.storiesSource, m.storiesTarget)
	return planner.BuildTargetFolderMap()
}

// findMissingFolderPaths analyzes selected items and identifies missing parent folders
func (m *Model) findMissingFolderPaths(items []PreflightItem) map[string]sb.Story {
	planner := sync.NewPreflightPlanner(m.storiesSource, m.storiesTarget)
	missingFolders := planner.FindMissingFolderPaths(items)

	// Convert slice to map for backward compatibility
	folderMap := make(map[string]sb.Story)
	for _, folder := range missingFolders {
		folderMap[folder.FullSlug] = folder
	}
	return folderMap
}

// optimizePreflight deduplicates entries, pre-plans missing folders, and sorts by sync order (folders first).
func (m *Model) optimizePreflight() {
	planner := sync.NewPreflightPlanner(m.storiesSource, m.storiesTarget)
	m.preflight.items = planner.OptimizePreflight(m.preflight.items)
}

func (m *Model) runNextItem() tea.Cmd {
	// Find next pending item, preferring current syncIndex, then scanning forward, then wrap-around
	if len(m.preflight.items) == 0 {
		return nil
	}
	idx := -1
	// First pass: from current index to end
	start := m.syncIndex
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.preflight.items); i++ {
		if m.preflight.items[i].Run == RunPending {
			idx = i
			break
		}
	}
	// Second pass: from 0 to current index
	if idx == -1 {
		for i := 0; i < start; i++ {
			if m.preflight.items[i].Run == RunPending {
				idx = i
				break
			}
		}
	}
	if idx == -1 {
		// No pending items — nothing to run
		return nil
	}
	m.syncIndex = idx
	m.preflight.items[idx].Run = RunRunning

	// Create orchestrator for this operation
	reportAdapter := &reportAdapter{report: &m.report}
	orchestrator := sync.NewSyncOrchestrator(m.api, reportAdapter, m.sourceSpace, m.targetSpace)

	// Create a sync item adapter
	item := &preflightItemAdapter{item: m.preflight.items[idx]}

	// Use orchestrator to run the sync operation
	return orchestrator.RunSyncItem(m.syncContext, idx, item)
}

// preflightItemAdapter adapts PreflightItem to sync.SyncItem interface
type preflightItemAdapter struct {
	item PreflightItem
}

func (pia *preflightItemAdapter) GetStory() sb.Story {
	return pia.item.Story
}

func (pia *preflightItemAdapter) IsFolder() bool {
	return pia.item.Story.IsFolder
}

// Legacy wrapper functions for backward compatibility
func isRateLimited(err error) bool {
	return sync.IsRateLimited(err)
}

func isDevModePublishLimit(err error) bool {
	return sync.IsDevModePublishLimit(err)
}

type updateAPI interface {
	UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error)
}

type createAPI interface {
	CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error)
}

// Legacy wrapper functions that now use the extracted API adapters
func updateStoryWithPublishRetry(ctx context.Context, api updateAPI, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	// Legacy wrapper removed; not used in raw path anymore
	return st, nil
}

func createStoryWithPublishRetry(ctx context.Context, api createAPI, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	// Legacy wrapper removed; not used in raw path anymore
	return st, nil
}

type folderAPI interface {
	GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error)
}

// folderPathBuilder handles the creation of folder hierarchies
type folderPathBuilder struct {
	api           folderAPI
	report        *Report
	sourceStories map[string]sb.Story
	contentMgr    *contentManager
	srcSpaceID    int
	tgtSpaceID    int
	publish       bool
}

// newFolderPathBuilder creates a new folder path builder
func newFolderPathBuilder(api folderAPI, report *Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, publish bool) *folderPathBuilder {
	// Build source stories map for quick lookup
	sourceMap := make(map[string]sb.Story)
	for _, story := range sourceStories {
		sourceMap[story.FullSlug] = story
	}

	return &folderPathBuilder{
		api:           api,
		report:        report,
		sourceStories: sourceMap,
		contentMgr:    newContentManager(api, srcSpaceID),
		srcSpaceID:    srcSpaceID,
		tgtSpaceID:    tgtSpaceID,
		publish:       publish,
	}
}

// checkExistingFolder checks if a folder exists in the target space
func (fpb *folderPathBuilder) checkExistingFolder(ctx context.Context, path string) (*sb.Story, error) {
	existing, err := fpb.api.GetStoriesBySlug(ctx, fpb.tgtSpaceID, path)
	if err != nil {
		return nil, err
	}

	if len(existing) == 0 {
		return nil, nil
	}

	folder := existing[0]
	log.Printf("DEBUG: Found existing folder: %s (ID: %d)", path, folder.ID)
	return &folder, nil
}

// prepareSourceFolder prepares a source folder for creation in target space
func (fpb *folderPathBuilder) prepareSourceFolder(ctx context.Context, path string, parentID *int) (sb.Story, error) {
	source, exists := fpb.sourceStories[path]
	if !exists {
		return sb.Story{}, fmt.Errorf("source folder not found: %s", path)
	}

	// Ensure content is loaded
	folder, err := fpb.contentMgr.ensureContent(ctx, source)
	if err != nil {
		log.Printf("DEBUG: Failed to fetch content for folder %s: %v", path, err)
		return sb.Story{}, err
	}

	// Prepare for creation
	folder = prepareStoryForCreation(folder)
	folder.FolderID = parentID

	log.Printf("DEBUG: Prepared source folder %s with content: %t", path, len(folder.Content) > 0)
	return folder, nil
}

// createFolder creates a single folder in the target space
func (fpb *folderPathBuilder) createFolder(ctx context.Context, folder sb.Story) (sb.Story, error) {
	log.Printf("DEBUG: Creating folder: %s", folder.FullSlug)
	// Convert typed folder to raw minimal for creation
	raw := map[string]interface{}{
		"uuid":      folder.UUID,
		"name":      folder.Name,
		"slug":      folder.Slug,
		"full_slug": folder.FullSlug,
		"content":   sync.ToRawMap(folder.Content),
		"is_folder": true,
	}
	if folder.FolderID != nil {
		raw["parent_id"] = *folder.FolderID
	} else {
		raw["parent_id"] = 0
	}
	created, err := fpb.api.CreateStoryRawWithPublish(ctx, fpb.tgtSpaceID, raw, false)
	if err != nil {
		log.Printf("DEBUG: Failed to create folder %s: %v", folder.FullSlug, err)
		return sb.Story{}, err
	}

	log.Printf("DEBUG: Successfully created folder %s (ID: %d)", created.FullSlug, created.ID)

	return created, nil
}

// ensureFolderPathImpl creates missing folders in a path hierarchy using modular approach
func ensureFolderPathImpl(api folderAPI, report *Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, slug string, publish bool) ([]sb.Story, error) {
	parts := strings.Split(slug, "/")
	if len(parts) <= 1 {
		return nil, nil
	}

	builder := newFolderPathBuilder(api, report, sourceStories, srcSpaceID, tgtSpaceID, publish)
	var created []sb.Story
	var parentID *int

	// Process each folder in the path hierarchy
	for i := 0; i < len(parts)-1; i++ {
		path := strings.Join(parts[:i+1], "/")

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

		// Check if folder already exists
		existing, err := builder.checkExistingFolder(ctx, path)
		cancel()

		if err != nil {
			return created, err
		}

		if existing != nil {
			// Folder exists, use its ID as parent for next level
			parentID = &existing.ID
			continue
		}

		// Folder doesn't exist, create it
		ctx, cancel = context.WithTimeout(context.Background(), defaultTimeout)
		folder, err := builder.prepareSourceFolder(ctx, path, parentID)
		cancel()

		if err != nil {
			return created, err
		}

		// Create the folder
		ctx, cancel = context.WithTimeout(context.Background(), defaultTimeout)
		createdFolder, err := builder.createFolder(ctx, folder)
		cancel()

		if err != nil {
			return created, err
		}

		created = append(created, createdFolder)
		parentID = &createdFolder.ID

		// Update report
		if report != nil {
			report.AddSuccess(createdFolder.FullSlug, operationCreate, 0, &createdFolder)
		}
	}

	return created, nil
}

func (m *Model) ensureFolderPath(slug string) ([]sb.Story, error) {
	return ensureFolderPathImpl(m.api, &m.report, m.storiesSource, m.sourceSpace.ID, m.targetSpace.ID, slug, m.shouldPublish())
}

func (m *Model) shouldPublish() bool {
	if m.targetSpace != nil && m.targetSpace.PlanLevel == 999 {
		return false
	}
	return true
}

// syncFolder handles folder synchronization with proper parent resolution
func (m *Model) syncFolder(sourceFolder sb.Story) error {
	log.Printf("Syncing folder: %s", sourceFolder.FullSlug)

	// Use the sourceFolder data directly, which should already have content
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fullFolder := sourceFolder

	// DEBUG: Log content preservation
	log.Printf("DEBUG: syncFolder %s has content: %t, is_folder: %t", sourceFolder.FullSlug, len(sourceFolder.Content) > 0, sourceFolder.IsFolder)
	if len(sourceFolder.Content) > 0 {
		contentKeys := sync.GetContentKeys(sourceFolder.Content)
		log.Printf("DEBUG: syncFolder source content keys: %v", contentKeys)

		// Special logging for content_types field
		if sourceFolder.IsFolder {
			if v, ok := sync.GetContentField(sourceFolder.Content, "content_types"); ok {
				log.Printf("DEBUG: syncFolder %s has content_types: %v", sourceFolder.FullSlug, v)
			} else {
				log.Printf("DEBUG: syncFolder %s missing content_types field", sourceFolder.FullSlug)
			}
		}
	}
	log.Printf("DEBUG: syncFolder %s ContentType field: '%s'", sourceFolder.FullSlug, sourceFolder.ContentType)

	// If the source folder doesn't have content, try to fetch it from API
	if len(fullFolder.Content) == 0 {
		apiFolder, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceFolder.ID)
		if err != nil {
			return err
		}
		// Preserve any content that came from the API
		if len(apiFolder.Content) > 0 {
			fullFolder.Content = apiFolder.Content
		} else {
			// Create minimal content structure for folders
			fullFolder.Content = json.RawMessage([]byte(`{}`))
		}
	}

	// Don't modify ContentType or Content - preserve exactly as from source

	// Check if folder already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceFolder.FullSlug)
	if err != nil {
		return err
	}

	// Resolve parent folder ID
	if fullFolder.FolderID != nil {
		parentSlug := parentSlug(fullFolder.FullSlug)
		if parentSlug != "" {
			if targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlug); err == nil && len(targetParents) > 0 {
				fullFolder.FolderID = &targetParents[0].ID
			} else {
				fullFolder.FolderID = nil // Set to root if parent not found
			}
		}
	}

	// Handle translated slugs
	fullFolder = m.processTranslatedSlugs(fullFolder, existing)

	if len(existing) > 0 {
		// Update existing folder
		existingFolder := existing[0]
		fullFolder.ID = existingFolder.ID
		// Never publish folders; update respects publish flag internally
		updated, err := updateStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullFolder, false)
		if err != nil {
			return err
		}

		// Update UUID if different
		if updated.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, updated.ID, fullFolder.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		log.Printf("Updated folder: %s", fullFolder.FullSlug)
	} else {
		// Create new folder
		// Clear ALL fields that shouldn't be set on creation (based on Storyblok CLI)
		fullFolder.ID = 0
		fullFolder.CreatedAt = ""
		fullFolder.UpdatedAt = "" // This was causing 422!

		// Note: Don't reset Position and FolderID here as they are set by parent resolution above

		// Ensure folders have proper content structure
		if fullFolder.IsFolder && len(fullFolder.Content) == 0 {
			fullFolder.Content = json.RawMessage([]byte(`{}`))
		}

		// Never publish folders on create
		created, err := createStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullFolder, false)
		if err != nil {
			return err
		}

		// Update UUID if different
		if created.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, created.ID, fullFolder.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for new folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		log.Printf("Created folder: %s", fullFolder.FullSlug)
	}

	return nil
}

// syncFolderDetailed handles folder synchronization and returns detailed results
func (m *Model) syncFolderDetailed(sourceFolder sb.Story) (*syncItemResult, error) {
	syncer := sync.NewStorySyncer(m.api, m.sourceSpace.ID, m.targetSpace.ID)
	return syncer.SyncFolderDetailed(sourceFolder, m.shouldPublish())
}

// executeSync has been moved to sync/api_adapters.go as ExecuteSync

// itemType returns a string describing the item type for logging
func itemType(story sb.Story) string {
	return sync.ItemType(story)
}

// syncStoryContent handles story synchronization with proper UUID management
func (m *Model) syncStoryContent(sourceStory sb.Story) error {
	log.Printf("Syncing story: %s", sourceStory.FullSlug)

	// Get full story content from source
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullStory, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceStory.ID)
	if err != nil {
		return err
	}

	// Check if story already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceStory.FullSlug)
	if err != nil {
		return err
	}

	// Resolve parent folder ID if story is in a folder
	if fullStory.FolderID != nil {
		parentSlug := parentSlug(fullStory.FullSlug)
		if parentSlug != "" {
			if targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlug); err == nil && len(targetParents) > 0 {
				fullStory.FolderID = &targetParents[0].ID
			} else {
				log.Printf("Warning: parent folder %s not found for story %s", parentSlug, fullStory.FullSlug)
				fullStory.FolderID = nil // Set to root if parent not found
			}
		}
	}

	// Handle translated slugs
	fullStory = m.processTranslatedSlugs(fullStory, existing)

	if len(existing) > 0 {
		// Update existing story
		existingStory := existing[0]
		fullStory.ID = existingStory.ID
		// Publish only if source was published and target space allows it
		publish := m.shouldPublish() && sourceStory.Published
		updated, err := updateStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullStory, publish)
		if err != nil {
			return err
		}

		// Update UUID if different
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Updated story: %s", fullStory.FullSlug)
	} else {
		// Create new story
		// Clear ALL fields that shouldn't be set on creation (based on Storyblok CLI)
		fullStory.ID = 0
		fullStory.CreatedAt = ""
		fullStory.UpdatedAt = "" // This was causing 422!

		// Note: Don't reset Position and FolderID here as they are set by parent resolution above

		// Ensure stories have content (required for Storyblok API)
		if !fullStory.IsFolder && len(fullStory.Content) == 0 {
			contentBytes, _ := json.Marshal(map[string]interface{}{
				"component": "page",
			})
			fullStory.Content = json.RawMessage(contentBytes)
		}

		// Publish only if source was published and target space allows it
		publish := m.shouldPublish() && sourceStory.Published
		created, err := createStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullStory, publish)
		if err != nil {
			return err
		}

		// Update UUID if different
		if created.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, created.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Created story: %s", fullStory.FullSlug)
	}

	return nil
}

// syncStoryContentDetailed handles story synchronization and returns detailed results
// Note: Folder structure is now pre-planned in optimizePreflight(), so no need to ensure folder path here
func (m *Model) syncStoryContentDetailed(sourceStory sb.Story) (*syncItemResult, error) {
	syncer := sync.NewStorySyncer(m.api, m.sourceSpace.ID, m.targetSpace.ID)
	return syncer.SyncStoryDetailed(sourceStory, m.shouldPublish())
}

// processTranslatedSlugs handles translated slug processing like the Storyblok CLI
func (m *Model) processTranslatedSlugs(sourceStory sb.Story, existingStories []sb.Story) sb.Story {
	return sync.ProcessTranslatedSlugs(sourceStory, existingStories)
}

// Starts-with bulk sync removed; prefix is a filter only.

// Legacy helper functions removed as bulk operations module was deleted.

// Legacy wrapper for parent slug extraction
func parentSlug(full string) string { return sync.ParentSlug(full) }

// Adapter functions to convert between UI and sync module types

// reportAdapter adapts UI Report to sync ReportInterface
type reportAdapter struct {
	report *Report
}

func (ra *reportAdapter) AddSuccess(slug, operation string, duration int64, story *sb.Story) {
	ra.report.AddSuccess(slug, operation, duration, story)
}

func (ra *reportAdapter) AddWarning(slug, operation, warning string, duration int64, sourceStory, targetStory *sb.Story) {
	ra.report.AddWarning(slug, operation, warning, duration, sourceStory, targetStory)
}

func (ra *reportAdapter) AddError(slug, operation string, duration int64, sourceStory *sb.Story, err error) {
	ra.report.AddError(slug, operation, err.Error(), duration, sourceStory)
}

// Removed type conversion helpers: UI now uses core PreflightItem directly
