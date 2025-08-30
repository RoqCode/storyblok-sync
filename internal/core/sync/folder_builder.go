package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"storyblok-sync/internal/sb"
)

// Timeout constants
const (
	DefaultTimeout = 15 * time.Second
)

// FolderAPI interface defines the methods needed for folder operations
type FolderAPI interface {
	GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error)
	CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error)
	UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error
}

// Report interface for folder creation reporting
type Report interface {
	AddSuccess(slug, operation string, duration int64, story *sb.Story)
}

// FolderPathBuilder handles the creation of folder hierarchies
type FolderPathBuilder struct {
	api        FolderAPI
	report     Report
	contentMgr *ContentManager
	srcSpaceID int
	tgtSpaceID int
	publish    bool
}

// NewFolderPathBuilder creates a new folder path builder
func NewFolderPathBuilder(api FolderAPI, report Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, publish bool) *FolderPathBuilder {
	// Note: We no longer rely on preloaded sourceStories for folder payloads, as the
	// list endpoint lacks full data. We'll fetch each folder segment from the API.
	return &FolderPathBuilder{
		api:        api,
		report:     report,
		contentMgr: NewContentManager(api, srcSpaceID),
		srcSpaceID: srcSpaceID,
		tgtSpaceID: tgtSpaceID,
		publish:    publish,
	}
}

// CheckExistingFolder checks if a folder exists in the target space
func (fpb *FolderPathBuilder) CheckExistingFolder(ctx context.Context, path string) (*sb.Story, error) {
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

// PrepareSourceFolder prepares a source folder for creation in target space
func (fpb *FolderPathBuilder) PrepareSourceFolder(ctx context.Context, path string, parentID *int) (map[string]interface{}, error) {
	// Look up the source folder by slug using API (not the preloaded map)
	matches, err := fpb.api.GetStoriesBySlug(ctx, fpb.srcSpaceID, path)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("source folder not found: %s", path)
	}
	source := matches[0]
	if !source.IsFolder {
		return nil, fmt.Errorf("expected folder at %s, found story", path)
	}

	// Fetch raw story payload from API to preserve unknown fields
	raw, err := fpb.api.GetStoryRaw(ctx, fpb.srcSpaceID, source.ID)
	if err != nil {
		log.Printf("DEBUG: Failed to fetch raw payload for folder %s: %v", path, err)
		return nil, err
	}

	// Omit raw payload dump to keep logs readable
	log.Printf("DEBUG: SOURCE_RAW folder %s (payload omitted)", path)

	// Strip read-only fields
	delete(raw, "id")
	delete(raw, "created_at")

	// Always set explicit parent_id (0 for root)
	if parentID == nil {
		raw["parent_id"] = 0
	} else {
		raw["parent_id"] = *parentID
	}

	// Convert translated_slugs -> translated_slugs_attributes (remove IDs)
	if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
		attrs := make([]map[string]interface{}, 0, len(ts))
		for _, item := range ts {
			if m, ok := item.(map[string]interface{}); ok {
				// remove id if present
				delete(m, "id")
				attrs = append(attrs, m)
			}
		}
		raw["translated_slugs_attributes"] = attrs
		delete(raw, "translated_slugs")
	}

	// Omit prepared payload dump
	log.Printf("DEBUG: PREPARED_RAW folder %s (payload omitted)", path)
	return raw, nil
}

// CreateFolder creates a single folder in the target space
func (fpb *FolderPathBuilder) CreateFolder(ctx context.Context, folder map[string]interface{}) (sb.Story, error) {
	slug := ""
	if v, ok := folder["full_slug"].(string); ok {
		slug = v
	}
	log.Printf("DEBUG: Creating folder: %s", slug)
	// Omit push raw payload dump
	log.Printf("DEBUG: PUSH_RAW folder %s (payload omitted)", slug)

	created, err := fpb.api.CreateStoryRawWithPublish(ctx, fpb.tgtSpaceID, folder, false /* never publish folders */)
	if err != nil {
		log.Printf("DEBUG: Failed to create folder %s: %v", slug, err)
		return sb.Story{}, err
	}

	log.Printf("DEBUG: Successfully created folder %s (ID: %d)", created.FullSlug, created.ID)

	// Update UUID after creation if source provided one
	if uuidVal, ok := folder["uuid"].(string); ok && uuidVal != "" && uuidVal != created.UUID {
		if err := fpb.api.UpdateStoryUUID(ctx, fpb.tgtSpaceID, created.ID, uuidVal); err != nil {
			log.Printf("DEBUG: Failed to update UUID for created folder %s: %v", slug, err)
		}
	}
	return created, nil
}

// EnsureFolderPath creates missing folders in a path hierarchy
func (fpb *FolderPathBuilder) EnsureFolderPath(slug string) ([]sb.Story, error) {
	parts := strings.Split(slug, "/")
	if len(parts) <= 1 {
		return nil, nil
	}

	var created []sb.Story
	var parentID *int

	// Process each folder in the path hierarchy
	for i := 0; i < len(parts)-1; i++ {
		path := strings.Join(parts[:i+1], "/")

		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)

		// Check if folder already exists
		existing, err := fpb.CheckExistingFolder(ctx, path)
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
		ctx, cancel = context.WithTimeout(context.Background(), DefaultTimeout)
		folder, err := fpb.PrepareSourceFolder(ctx, path, parentID)
		cancel()

		if err != nil {
			return created, err
		}

		// Create the folder
		ctx, cancel = context.WithTimeout(context.Background(), DefaultTimeout)
		createdFolder, err := fpb.CreateFolder(ctx, folder)
		cancel()

		if err != nil {
			return created, err
		}

		created = append(created, createdFolder)
		parentID = &createdFolder.ID

		// Update report
		if fpb.report != nil {
			fpb.report.AddSuccess(createdFolder.FullSlug, OperationCreate, 0, &createdFolder)
		}
	}

	return created, nil
}

// EnsureFolderPathStatic is a static utility function for ensuring folder paths
func EnsureFolderPathStatic(api FolderAPI, report Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, slug string, publish bool) ([]sb.Story, error) {
	builder := NewFolderPathBuilder(api, report, sourceStories, srcSpaceID, tgtSpaceID, publish)
	return builder.EnsureFolderPath(slug)
}
