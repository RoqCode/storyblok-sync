package sync

import (
	"storyblok-sync/internal/sb"
)

// SyncItemResult represents the result of a single sync operation
type SyncItemResult struct {
	Operation   string    `json:"operation"`   // create|update|skip
	TargetStory *sb.Story `json:"targetStory"` // created/updated story
	Warning     string    `json:"warning"`     // any warnings
}

// SyncResultMsg represents a message containing sync operation results
type SyncResultMsg struct {
	Index     int             `json:"index"`
	Err       error           `json:"error"`
	Result    *SyncItemResult `json:"result"`
	Duration  int64           `json:"duration"`  // in milliseconds
	Cancelled bool            `json:"cancelled"` // true if operation was cancelled
}

// SyncCancelledMsg represents a cancellation message
type SyncCancelledMsg struct {
	Message string `json:"message"`
}

// FolderMap represents a mapping of folder paths to stories
type FolderMap map[string]sb.Story
