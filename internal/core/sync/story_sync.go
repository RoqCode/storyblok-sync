package sync

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"storyblok-sync/internal/sb"
)

// StorySyncer handles story and folder synchronization operations
type StorySyncer struct {
	api           SyncAPI
	contentMgr    *ContentManager
	sourceSpaceID int
	targetSpaceID int
}

// storyRawAPI captures optional raw story methods available on the API client
type storyRawAPI interface {
	GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error)
	CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error)
	UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error)
}

// NewStorySyncer creates a new story synchronizer
func NewStorySyncer(api SyncAPI, sourceSpaceID, targetSpaceID int) *StorySyncer {
	return &StorySyncer{
		api:           api,
		contentMgr:    NewContentManager(api, sourceSpaceID),
		sourceSpaceID: sourceSpaceID,
		targetSpaceID: targetSpaceID,
	}
}

// SyncStory synchronizes a single story
func (ss *StorySyncer) SyncStory(ctx context.Context, story sb.Story, shouldPublish bool) (sb.Story, error) {
	log.Printf("Syncing story: %s", story.FullSlug)

	// Ensure content is loaded
	fullStory, err := ss.contentMgr.EnsureContent(ctx, story)
	if err != nil {
		return sb.Story{}, err
	}

	// DEBUG: minimal info about typed content presence
	log.Printf("DEBUG: SOURCE_TYPED content present for %s: %t", story.FullSlug, len(fullStory.Content) > 0)

	// Ensure non-folder stories have default content
	fullStory = EnsureDefaultContent(fullStory)

	// Check if story already exists in target
	existing, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, story.FullSlug)
	if err != nil {
		return sb.Story{}, err
	}

	// Resolve parent folder ID if needed
	fullStory = ss.resolveParentFolder(ctx, fullStory)

	// Handle translated slugs
	fullStory = ProcessTranslatedSlugs(fullStory, existing)

	if len(existing) > 0 {
		// Update existing story
		existingStory := existing[0]

		// Prefer raw update if available to preserve unknown fields
		if rawAPI, ok := any(ss.api).(storyRawAPI); ok {
			// Fetch raw source payload
			raw, err := rawAPI.GetStoryRaw(ctx, ss.sourceSpaceID, story.ID)
			if err != nil {
				return sb.Story{}, err
			}

			// Strip read-only fields and ensure correct parent_id
			delete(raw, "id")
			delete(raw, "created_at")
			delete(raw, "updated_at")
			if fullStory.FolderID != nil {
				raw["parent_id"] = *fullStory.FolderID
			} else {
				raw["parent_id"] = 0
			}

			// Translate translated_slugs -> translated_slugs_attributes without IDs
			if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
				attrs := make([]map[string]interface{}, 0, len(ts))
				for _, item := range ts {
					if m, ok := item.(map[string]interface{}); ok {
						delete(m, "id")
						attrs = append(attrs, m)
					}
				}
				raw["translated_slugs_attributes"] = attrs
				delete(raw, "translated_slugs")
			}

			// DEBUG: omit raw payload dump to keep logs readable
			log.Printf("DEBUG: PUSH_RAW_UPDATE story %s (payload omitted)", story.FullSlug)

			updated, err := rawAPI.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, existingStory.ID, raw, shouldPublish)
			if err != nil {
				return sb.Story{}, err
			}

			// Update UUID if different
			if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
				if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
					log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
				}
			}

			log.Printf("Updated story: %s", fullStory.FullSlug)
			return updated, nil
		}

		// Fallback to typed update
		updateStory := PrepareStoryForUpdate(fullStory, existingStory)
		// DEBUG: omit typed payload dump to keep logs readable
		log.Printf("DEBUG: PUSH_TYPED_UPDATE story %s (payload omitted)", story.FullSlug)
		updated, err := ss.api.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, existingStory.ID, map[string]interface{}{"uuid": updateStory.UUID, "name": updateStory.Name, "slug": updateStory.Slug, "full_slug": updateStory.FullSlug, "content": toMap(updateStory.Content), "is_folder": updateStory.IsFolder, "parent_id": valueOrZero(updateStory.FolderID)}, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		// Update UUID if different
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		// Update UUID if different after update
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Updated story: %s", fullStory.FullSlug)
		return updated, nil
	} else {
		// Create new story
		// Prefer raw create if available to preserve unknown fields
		if rawAPI, ok := any(ss.api).(storyRawAPI); ok {
			// Fetch raw source payload
			raw, err := rawAPI.GetStoryRaw(ctx, ss.sourceSpaceID, story.ID)
			if err != nil {
				return sb.Story{}, err
			}

			// Strip read-only fields
			delete(raw, "id")
			delete(raw, "created_at")
			delete(raw, "updated_at")

			// Set parent_id from resolved target parent
			if fullStory.FolderID != nil {
				raw["parent_id"] = *fullStory.FolderID
			} else {
				raw["parent_id"] = 0
			}

			// Translate translated_slugs -> translated_slugs_attributes without IDs
			if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
				attrs := make([]map[string]interface{}, 0, len(ts))
				for _, item := range ts {
					if m, ok := item.(map[string]interface{}); ok {
						delete(m, "id")
						attrs = append(attrs, m)
					}
				}
				raw["translated_slugs_attributes"] = attrs
				delete(raw, "translated_slugs")
			}

			// DEBUG: omit raw create payload dump
			log.Printf("DEBUG: PUSH_RAW_CREATE story %s (payload omitted)", story.FullSlug)

			created, err := rawAPI.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, raw, shouldPublish)
			if err != nil {
				return sb.Story{}, err
			}

			// Update UUID if different after create
			if created.UUID != fullStory.UUID && fullStory.UUID != "" {
				if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, created.ID, fullStory.UUID); err != nil {
					log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
				}
			}

			log.Printf("Created story: %s", fullStory.FullSlug)
			return created, nil
		}

		// Fallback to typed create
		createStory := PrepareStoryForCreation(fullStory)

		// DEBUG: omit typed create payload dump
		log.Printf("DEBUG: PUSH_TYPED_CREATE story %s (payload omitted)", story.FullSlug)

		created, err := ss.api.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, map[string]interface{}{"uuid": createStory.UUID, "name": createStory.Name, "slug": createStory.Slug, "full_slug": createStory.FullSlug, "content": toMap(createStory.Content), "is_folder": createStory.IsFolder, "parent_id": valueOrZero(createStory.FolderID)}, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		// Update UUID if different after create
		if created.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, created.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Created story: %s", fullStory.FullSlug)
		return created, nil
	}
}

// toMap converts a JSON blob to map[string]interface{} for raw payloads
func toMap(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// valueOrZero returns the dereferenced int or 0 if nil
func valueOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// SyncFolder synchronizes a single folder
func (ss *StorySyncer) SyncFolder(ctx context.Context, folder sb.Story, shouldPublish bool) (sb.Story, error) {
	log.Printf("Syncing folder: %s", folder.FullSlug)

	// Ensure content is loaded for folder
	fullFolder, err := ss.contentMgr.EnsureContent(ctx, folder)
	if err != nil {
		// If content loading fails, use folder as-is with minimal content
		fullFolder = folder
		if len(fullFolder.Content) == 0 {
			fullFolder.Content = json.RawMessage([]byte(`{}`))
		}
	}

	// Debug logging
	log.Printf("DEBUG: syncFolder %s has content: %t, is_folder: %t",
		folder.FullSlug, len(fullFolder.Content) > 0, folder.IsFolder)

	// Check if folder already exists in target
	existing, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, folder.FullSlug)
	if err != nil {
		return sb.Story{}, err
	}

	// Resolve parent folder ID if needed
	fullFolder = ss.resolveParentFolder(ctx, fullFolder)

	// Handle translated slugs
	fullFolder = ProcessTranslatedSlugs(fullFolder, existing)

	if len(existing) > 0 {
		// Update existing folder
		existingFolder := existing[0]
		updateFolder := PrepareStoryForUpdate(fullFolder, existingFolder)

		updated, err := ss.api.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, existingFolder.ID, map[string]interface{}{"uuid": updateFolder.UUID, "name": updateFolder.Name, "slug": updateFolder.Slug, "full_slug": updateFolder.FullSlug, "content": toMap(updateFolder.Content), "is_folder": true, "parent_id": valueOrZero(updateFolder.FolderID)}, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		// Update UUID if different
		if updated.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullFolder.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		log.Printf("Updated folder: %s", fullFolder.FullSlug)
		return updated, nil
	} else {
		// Create new folder
		// Prefer raw create if available to preserve unknown folder fields
		if rawAPI, ok := any(ss.api).(storyRawAPI); ok {
			// Fetch raw source payload
			raw, err := rawAPI.GetStoryRaw(ctx, ss.sourceSpaceID, folder.ID)
			if err != nil {
				return sb.Story{}, err
			}

			// Strip read-only fields
			delete(raw, "id")
			delete(raw, "created_at")
			delete(raw, "updated_at")

			// Ensure is_folder true
			raw["is_folder"] = true

			// Set parent_id from resolved target parent (already computed in fullFolder)
			if fullFolder.FolderID != nil {
				raw["parent_id"] = *fullFolder.FolderID
			} else {
				raw["parent_id"] = 0
			}

			// Translate translated_slugs -> translated_slugs_attributes without IDs
			if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
				attrs := make([]map[string]interface{}, 0, len(ts))
				for _, item := range ts {
					if m, ok := item.(map[string]interface{}); ok {
						delete(m, "id")
						attrs = append(attrs, m)
					}
				}
				raw["translated_slugs_attributes"] = attrs
				delete(raw, "translated_slugs")
			}

			// DEBUG: log outgoing raw create payload for folder
			if b, err := json.MarshalIndent(raw, "", "  "); err == nil {
				log.Printf("DEBUG: PUSH_RAW_CREATE folder %s:\n%s", fullFolder.FullSlug, string(b))
			}

			created, err := rawAPI.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, raw, false)
			if err != nil {
				return sb.Story{}, err
			}

			log.Printf("Created folder: %s", fullFolder.FullSlug)
			return created, nil
		}

		// Fallback to typed folder create
		createFolder := PrepareStoryForCreation(fullFolder)
		if createFolder.IsFolder && len(createFolder.Content) == 0 {
			createFolder.Content = json.RawMessage([]byte(`{}`))
		}
		created, err := ss.api.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, map[string]interface{}{"uuid": createFolder.UUID, "name": createFolder.Name, "slug": createFolder.Slug, "full_slug": createFolder.FullSlug, "content": toMap(createFolder.Content), "is_folder": true, "parent_id": valueOrZero(createFolder.FolderID)}, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}
		log.Printf("Created folder: %s", fullFolder.FullSlug)
		return created, nil
	}
}

// SyncStoryDetailed synchronizes a story and returns detailed result
func (ss *StorySyncer) SyncStoryDetailed(story sb.Story, shouldPublish bool) (*SyncItemResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Determine operation type based on whether story exists BEFORE syncing
	operation := OperationCreate
	if existing, _ := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, story.FullSlug); len(existing) > 0 {
		operation = OperationUpdate
	}

	targetStory, err := ss.SyncStory(ctx, story, shouldPublish)
	if err != nil {
		return nil, err
	}

	return &SyncItemResult{
		Operation:   operation,
		TargetStory: &targetStory,
		Warning:     "",
	}, nil
}

// SyncFolderDetailed synchronizes a folder and returns detailed result
func (ss *StorySyncer) SyncFolderDetailed(folder sb.Story, shouldPublish bool) (*SyncItemResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Determine operation type based on whether folder exists BEFORE syncing
	operation := OperationCreate
	if existing, _ := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, folder.FullSlug); len(existing) > 0 {
		operation = OperationUpdate
	}

	targetFolder, err := ss.SyncFolder(ctx, folder, shouldPublish)
	if err != nil {
		return nil, err
	}

	return &SyncItemResult{
		Operation:   operation,
		TargetStory: &targetFolder,
		Warning:     "",
	}, nil
}

// resolveParentFolder resolves and sets the correct parent folder ID for a story
func (ss *StorySyncer) resolveParentFolder(ctx context.Context, story sb.Story) sb.Story {
	if story.FolderID == nil {
		return story
	}

	parentSlugStr := ParentSlug(story.FullSlug)
	if parentSlugStr == "" {
		story.FolderID = nil
		return story
	}

	targetParents, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, parentSlugStr)
	if err != nil {
		log.Printf("Warning: failed to resolve parent folder for %s: %v", story.FullSlug, err)
		return story
	}

	if len(targetParents) > 0 {
		story.FolderID = &targetParents[0].ID
	} else {
		story.FolderID = nil
		log.Printf("Warning: Parent folder %s not found in target space for %s", parentSlugStr, story.FullSlug)
	}

	return story
}
