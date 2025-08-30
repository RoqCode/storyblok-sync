package sync

import (
	"log"
	"sort"
	"strings"

	"storyblok-sync/internal/sb"
)

// PreflightItem represents an item in the preflight plan (using core types)
type PreflightItem struct {
	Story     sb.Story
	Collision bool
	Skip      bool
	Selected  bool
	State     string // StateCreate, StateUpdate, etc.
	Run       string // RunPending, RunRunning, etc.
	// Optional UI-only field for inline messages
	Issue string
}

// State constants for preflight items
const (
	StateCreate = "create"
	StateUpdate = "update"
	StateSkip   = "skip"
)

// Run state constants
const (
	RunPending = "pending"
	RunRunning = "running"
	RunSuccess = "success"
	RunFailed  = "failed"
)

// PreflightPlanner handles preflight planning and optimization
type PreflightPlanner struct {
	sourceStories []sb.Story
	targetStories []sb.Story
}

// NewPreflightPlanner creates a new preflight planner
func NewPreflightPlanner(sourceStories, targetStories []sb.Story) *PreflightPlanner {
	return &PreflightPlanner{
		sourceStories: sourceStories,
		targetStories: targetStories,
	}
}

// OptimizePreflight deduplicates and sorts preflight items for optimal sync order
func (pp *PreflightPlanner) OptimizePreflight(items []PreflightItem) []PreflightItem {
	log.Printf("Optimizing preflight with %d items", len(items))

	// Deduplicate by FullSlug
	selected := make(map[string]*PreflightItem)
	for i := range items {
		it := &items[i]
		if it.Skip {
			continue
		}
		if _, ok := selected[it.Story.FullSlug]; ok {
			it.Skip = true
			continue
		}
		selected[it.Story.FullSlug] = it
	}

	// Create initial optimized list
	optimized := make([]PreflightItem, 0, len(items))
	for _, it := range items {
		if it.Skip {
			continue
		}
		it.Run = RunPending
		optimized = append(optimized, it)
	}

	// Find and add missing folder paths
	missingFolders := pp.FindMissingFolderPaths(optimized)
	log.Printf("Found %d missing folders that need to be created", len(missingFolders))

	// Build a map of already included slugs to avoid duplicates
	existingSlugs := make(map[string]bool)
	for _, item := range optimized {
		existingSlugs[item.Story.FullSlug] = true
	}

	// Add missing folders to the plan
	for _, folder := range missingFolders {
		// Skip if folder is already included in the optimization list
		if existingSlugs[folder.FullSlug] {
			log.Printf("DEBUG: Folder %s already in optimization list, skipping auto-add", folder.FullSlug)
			continue
		}

		// Create preflight item for missing folder
		folderItem := PreflightItem{
			Story:     folder,
			Collision: false, // Missing folders don't have collisions
			Skip:      false,
			Selected:  true, // Auto-selected for sync
			State:     StateCreate,
			Run:       RunPending,
		}
		optimized = append(optimized, folderItem)
		existingSlugs[folder.FullSlug] = true
		log.Printf("DEBUG: Auto-added missing folder to preflight: %s", folder.FullSlug)
	}

	// Sort by sync priority: folders first (by depth), then stories
	sort.Slice(optimized, func(i, j int) bool {
		itemI, itemJ := optimized[i], optimized[j]

		// Folders always come before stories
		if itemI.Story.IsFolder && !itemJ.Story.IsFolder {
			return true
		}
		if !itemI.Story.IsFolder && itemJ.Story.IsFolder {
			return false
		}

		// Both are folders or both are stories - sort by depth (shallow first)
		depthI := strings.Count(itemI.Story.FullSlug, "/")
		depthJ := strings.Count(itemJ.Story.FullSlug, "/")

		if depthI != depthJ {
			return depthI < depthJ
		}

		// Same depth - sort alphabetically for consistent order
		return itemI.Story.FullSlug < itemJ.Story.FullSlug
	})

	// Update the list
	log.Printf("Optimized to %d items (%d missing folders auto-added), sync order: folders first, then stories",
		len(optimized), len(missingFolders))

	return optimized
}

// FindMissingFolderPaths analyzes preflight items and finds missing parent folders
func (pp *PreflightPlanner) FindMissingFolderPaths(items []PreflightItem) []sb.Story {
	// Build target folder map for quick lookup
	targetFolderMap := pp.BuildTargetFolderMap()

	missingPaths := make(map[string]bool)

	// For each item, check if all parent folders exist
	for _, item := range items {
		// Get all parent paths for this story
		folderPaths := GetFolderPaths(item.Story.FullSlug)

		for _, path := range folderPaths {
			// Check if this folder path exists in target
			if _, exists := targetFolderMap[path]; !exists {
				missingPaths[path] = true
			}
		}
	}

	// Convert missing paths to stories by finding them in source
	var missingFolders []sb.Story
	sourceMap := make(map[string]sb.Story)
	for _, story := range pp.sourceStories {
		sourceMap[story.FullSlug] = story
	}

	for path := range missingPaths {
		if folder, exists := sourceMap[path]; exists && folder.IsFolder {
			log.Printf("DEBUG: Found missing folder path: %s", path)
			missingFolders = append(missingFolders, folder)
		}
	}

	return missingFolders
}

// BuildTargetFolderMap creates a map of existing folders in target space for quick lookup
func (pp *PreflightPlanner) BuildTargetFolderMap() map[string]sb.Story {
	folderMap := make(map[string]sb.Story)
	for _, story := range pp.targetStories {
		if story.IsFolder {
			folderMap[story.FullSlug] = story
		}
	}
	return folderMap
}

// ProcessTranslatedSlugs handles translated slug processing like the Storyblok CLI
func ProcessTranslatedSlugs(sourceStory sb.Story, existingStories []sb.Story) sb.Story {
	if len(sourceStory.TranslatedSlugs) == 0 {
		return sourceStory
	}

	// Copy translated slugs and remove IDs
	translatedSlugs := make([]sb.TranslatedSlug, len(sourceStory.TranslatedSlugs))
	for i, ts := range sourceStory.TranslatedSlugs {
		translatedSlugs[i] = sb.TranslatedSlug{
			Lang: ts.Lang,
			Name: ts.Name,
			Path: ts.Path,
		}
	}

	// If there's an existing story, merge the translated slug IDs
	if len(existingStories) > 0 {
		existingStory := existingStories[0]
		if len(existingStory.TranslatedSlugs) > 0 {
			for i := range translatedSlugs {
				for _, existingTS := range existingStory.TranslatedSlugs {
					if translatedSlugs[i].Lang == existingTS.Lang {
						translatedSlugs[i].ID = existingTS.ID
						break
					}
				}
			}
		}
	}

	// Set the attributes for the API call
	sourceStory.TranslatedSlugsAttributes = translatedSlugs
	sourceStory.TranslatedSlugs = nil // Clear the original field

	return sourceStory
}

// ParentSlug extracts the parent folder slug from a full slug
func ParentSlug(slug string) string {
	slashIdx := strings.LastIndex(slug, "/")
	if slashIdx == -1 {
		return ""
	}
	return slug[:slashIdx]
}

// ItemType returns a string describing the item type for logging
func ItemType(story sb.Story) string {
	if story.IsFolder {
		return "folder"
	}
	return "story"
}
