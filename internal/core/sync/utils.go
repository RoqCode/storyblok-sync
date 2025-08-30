package sync

import (
	"encoding/json"
	"log"
	"strings"

	"storyblok-sync/internal/sb"
)

// Constants for sync operations
const (
	DefaultComponent = "page"
	OperationCreate  = "create"
	OperationUpdate  = "update"
	OperationSkip    = "skip"
)

// PrepareStoryForCreation prepares a story for creation by clearing read-only fields
func PrepareStoryForCreation(story sb.Story) sb.Story {
	story.ID = 0
	story.CreatedAt = ""
	story.UpdatedAt = ""
	return story
}

// PrepareStoryForUpdate prepares a story for update by preserving necessary fields
func PrepareStoryForUpdate(source, target sb.Story) sb.Story {
	// Keep target's ID and timestamps, but use source's content
	source.ID = target.ID
	source.CreatedAt = target.CreatedAt
	// Don't set UpdatedAt - let API handle it
	source.UpdatedAt = ""
	return source
}

// EnsureDefaultContent ensures non-folder stories have content
func EnsureDefaultContent(story sb.Story) sb.Story {
	if !story.IsFolder && len(story.Content) == 0 {
		// {"component":"page"}
		contentBytes, _ := json.Marshal(map[string]interface{}{
			"component": DefaultComponent,
		})
		story.Content = json.RawMessage(contentBytes)
	}
	return story
}

// GetContentKeys extracts keys from a JSON content blob for debugging
func GetContentKeys(content json.RawMessage) []string {
	if len(content) == 0 {
		return nil
	}
	var tmp map[string]interface{}
	if err := json.Unmarshal(content, &tmp); err != nil {
		return nil
	}
	keys := make([]string, 0, len(tmp))
	for k := range tmp {
		keys = append(keys, k)
	}
	return keys
}

// GetContentField returns an arbitrary field from JSON content as interface{}
func GetContentField(content json.RawMessage, key string) (interface{}, bool) {
	if len(content) == 0 {
		return nil, false
	}
	var tmp map[string]interface{}
	if err := json.Unmarshal(content, &tmp); err != nil {
		return nil, false
	}
	v, ok := tmp[key]
	return v, ok
}

// ToRawMap converts a json.RawMessage into a map for raw payloads
func ToRawMap(content json.RawMessage) map[string]interface{} {
	if len(content) == 0 {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(content, &m)
	return m
}

// GetFolderPaths extracts all parent folder paths from a story slug
func GetFolderPaths(slug string) []string {
	parts := strings.Split(slug, "/")
	if len(parts) <= 1 {
		return nil
	}

	paths := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		path := strings.Join(parts[:i], "/")
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// LogError logs comprehensive error information for debugging
func LogError(operation, slug string, err error, story *sb.Story) {
	log.Printf("ERROR: %s failed for %s: %v", operation, slug, err)

	if story != nil {
		// Log story context
		log.Printf("ERROR CONTEXT for %s:", slug)
		log.Printf("  Story ID: %d", story.ID)
		log.Printf("  Story UUID: %s", story.UUID)
		log.Printf("  Story Name: %s", story.Name)
		log.Printf("  Full Slug: %s", story.FullSlug)
		log.Printf("  Is Folder: %t", story.IsFolder)
		log.Printf("  Published: %t", story.Published)

		if story.FolderID != nil {
			log.Printf("  Parent ID: %d", *story.FolderID)
		}

		if len(story.TagList) > 0 {
			log.Printf("  Tags: %v", story.TagList)
		}

		if len(story.TranslatedSlugs) > 0 {
			log.Printf("  Translated Slugs: %d entries", len(story.TranslatedSlugs))
			for _, ts := range story.TranslatedSlugs {
				log.Printf("    - %s: %s (%s)", ts.Lang, ts.Name, ts.Path)
			}
		}

		// Log content summary (first level keys only, to avoid huge logs)
		if len(story.Content) > 0 {
			contentKeys := GetContentKeys(story.Content)
			log.Printf("  Content Keys: %v", contentKeys)

			// Log component type if available
			if v, ok := GetContentField(story.Content, "component"); ok {
				if component, _ := v.(string); component != "" {
					log.Printf("  Component Type: %s", component)
				}
			}
		}

		// Log full story as JSON for complete debugging (only if content is small enough)
		if storyJSON, err := json.Marshal(story); err == nil {
			if len(storyJSON) < 2000 { // Only log if less than 2KB
				log.Printf("  Full Story JSON: %s", string(storyJSON))
			} else {
				log.Printf("  Full Story JSON: [too large, %d bytes - see report file]", len(storyJSON))
			}
		}
	}

	// Log additional error context if available
	logExtendedErrorContext(err)
}

// LogWarning logs comprehensive warning information
func LogWarning(operation, slug, warning string, story *sb.Story) {
	log.Printf("WARNING: %s for %s: %s", operation, slug, warning)

	if story != nil {
		log.Printf("WARNING CONTEXT for %s:", slug)
		log.Printf("  Story ID: %d (UUID: %s)", story.ID, story.UUID)
		log.Printf("  Full Slug: %s", story.FullSlug)
		if story.FolderID != nil {
			log.Printf("  Parent ID: %d", *story.FolderID)
		}
	}
}

// LogSuccess logs success with context information
func LogSuccess(operation, slug string, duration int64, targetStory *sb.Story) {
	log.Printf("SUCCESS: %s completed for %s in %dms", operation, slug, duration)

	if targetStory != nil {
		log.Printf("SUCCESS CONTEXT for %s:", slug)
		log.Printf("  Created/Updated Story ID: %d (UUID: %s)", targetStory.ID, targetStory.UUID)
		if targetStory.FolderID != nil {
			log.Printf("  Parent ID: %d", *targetStory.FolderID)
		}
		log.Printf("  Published: %t", targetStory.Published)
	}
}

// logExtendedErrorContext extracts and logs additional context from errors
func logExtendedErrorContext(err error) {
	if err == nil {
		return
	}

	errStr := err.Error()

	// Check for common API error patterns and log additional context
	if strings.Contains(errStr, "status") {
		log.Printf("  HTTP Error Details: %s", errStr)
	}

	if strings.Contains(errStr, "timeout") {
		log.Printf("  Timeout Error - this may indicate network issues or server overload")
	}

	if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
		log.Printf("  Authentication/Authorization Error - check token permissions")
	}

	if strings.Contains(errStr, "404") {
		log.Printf("  Resource Not Found - story or space may not exist")
	}

	if strings.Contains(errStr, "429") {
		log.Printf("  Rate Limited - will retry with backoff")
	}

	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") {
		log.Printf("  Server Error - this may be temporary, will retry")
	}
}
