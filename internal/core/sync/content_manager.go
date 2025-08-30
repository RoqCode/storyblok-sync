package sync

import (
	"context"

	"encoding/json"
	"storyblok-sync/internal/sb"
)

// folderAPI interface defines the methods needed for content management
type folderAPI interface {
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
}

// ContentManager handles story content fetching and caching
type ContentManager struct {
	api      folderAPI
	spaceID  int
	cache    map[int]sb.Story
	maxSize  int
	hitCount int
}

// NewContentManager creates a new content manager with cache size limit
func NewContentManager(api folderAPI, spaceID int) *ContentManager {
	return &ContentManager{
		api:     api,
		spaceID: spaceID,
		cache:   make(map[int]sb.Story),
		maxSize: 500, // Limit cache to 500 entries
	}
}

// EnsureContent fetches story content if not present, with caching
func (cm *ContentManager) EnsureContent(ctx context.Context, story sb.Story) (sb.Story, error) {
	// Return if content already exists
	if len(story.Content) > 0 {
		return story, nil
	}

	// Check cache first
	if cached, exists := cm.cache[story.ID]; exists && len(cached.Content) > 0 {
		cm.hitCount++
		story.Content = cached.Content
		return story, nil
	}

	// Fetch from API
	fullStory, err := cm.api.GetStoryWithContent(ctx, cm.spaceID, story.ID)
	if err != nil {
		return story, err
	}

	// Use fetched content or default
	if len(fullStory.Content) > 0 {
		story.Content = fullStory.Content
	} else {
		story.Content = json.RawMessage([]byte(`{}`))
	}

	// Cache the result with size limit
	cm.addToCache(story)
	return story, nil
}

// addToCache adds a story to the cache with LRU eviction when size limit is reached
func (cm *ContentManager) addToCache(story sb.Story) {
	// If cache is at capacity, remove oldest entries to make room
	if len(cm.cache) >= cm.maxSize {
		// Simple eviction: remove entries until we're under the limit
		removeCount := len(cm.cache) - cm.maxSize + 1
		count := 0
		for id := range cm.cache {
			if count >= removeCount {
				break
			}
			delete(cm.cache, id)
			count++
		}
	}
	cm.cache[story.ID] = story
}

// CacheStats returns cache statistics
func (cm *ContentManager) CacheStats() (size, maxSize int) {
	return len(cm.cache), cm.maxSize
}

// ClearCache clears the entire cache
func (cm *ContentManager) ClearCache() {
	cm.cache = make(map[int]sb.Story)
}
