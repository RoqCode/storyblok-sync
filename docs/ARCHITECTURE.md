# Architecture & Folder Structure

This document summarizes the project layout, module responsibilities, and the boundaries between UI, core domain logic, and infrastructure.

Related docs:
- Top‑level README (features, user flow): [../README.md](../README.md)
- Planning and roadmap details: [PLANNING.md](./PLANNING.md)

## Overview

The app is a terminal UI (Bubble Tea) that synchronizes Stories and Folders between two Storyblok spaces. It is structured to keep the UI concerns separate from the domain sync core and the Storyblok API client.

```
storyblok-sync/
├─ cmd/sbsync/              # Application entry (main package)
├─ internal/
│  ├─ config/               # Token/config load & save (no Storyblok logic)
│  ├─ sb/                   # Storyblok API client (pure HTTP, typed+raw)
│  ├─ ui/                   # Bubble Tea TUI (state, views, inputs)
│  └─ core/
│     └─ sync/              # Domain sync core (planner/orchestrator/syncer)
└─ docs/                    # Docs and plans
```

## Modules & Responsibilities

- `cmd/sbsync/`:
  - Program bootstrap, DEBUG logging configuration, and Bubble Tea program startup.
  - No business logic here.

- `internal/ui/` (Bubble Tea UI):
  - Implements the MVU model (state, update handlers, views) for:
    - Auth/token, space selection, scanning, browse/search, preflight, sync, and report.
  - Delegates domain operations to the core sync via small adapters.
  - Renders state using `PreflightItem` from the core (unified type).
  - Must not call HTTP directly; talks only to `internal/sb.Client` via the core orchestrator.

- `internal/core/sync/` (Domain core):
  - Business logic for sync, extracted from the UI to keep concerns separate.
  - Key components:
    - `PreflightPlanner`: de‑duplicates, auto‑adds missing folders, sorts folders before stories.
    - `FolderPathBuilder`: ensures folder chains exist in the target using raw payloads.
    - `StorySyncer`: create/update logic for stories and folders.
    - `SyncOrchestrator`: runs sync operations with retries and reports progress back to the UI.
    - `ContentManager`: ensures full content is loaded and caches results.
    - `types.go`: message/result types used during sync.
    - `utils.go`: helpers (translated slugs processing, default content, logging, path helpers).
  - Depends on `internal/sb` interfaces only (no UI imports).

- `internal/sb/` (Storyblok API client):
  - Typed Story model + raw read/write accessors to preserve unknown fields.
  - Methods used by the core: `GetStoriesBySlug`, `GetStoryWithContent`, `GetStoryRaw`, `CreateStoryRawWithPublish`, `UpdateStoryRawWithPublish`, `UpdateStoryUUID`.
  - No UI logic; returns errors with enough context for the core to retry or report.

- `internal/config/`:
  - Load and persist local config/token in a safe place; no secrets in VCS.

## Data Flow

1. UI gathers selection and builds a `[]PreflightItem` (core type).
2. `PreflightPlanner` optimizes the list (dedupe, add missing folders, order folders first).
3. `SyncOrchestrator` executes items with retry and reporting.
4. For each story:
   - `EnsureFolderPathStatic` creates missing target folders via `FolderPathBuilder` (raw payloads).
   - `StorySyncer.SyncStory`:
     - Loads content (typed) only if missing.
     - For raw path: fetches source raw, strips read‑only fields, sets `parent_id`, converts `translated_slugs` → `translated_slugs_attributes`, writes to target (create/update), and then aligns UUID.
     - For typed fallback: constructs a minimal raw map from typed Story and writes it.

## Raw Story Handling (Invariants)

To match Storyblok CLI behavior and preserve unknown fields:
- Always prefer raw get + raw write for stories and folders.
- Before write: remove `id/created_at/updated_at`, set `parent_id` (0 if absent), convert `translated_slugs` to `translated_slugs_attributes` with IDs removed.
- Never publish folders; publish stories according to plan/policy.
- After write, if target UUID differs and source UUID is present, update via `UpdateStoryUUID`.

These invariants are validated by tests in `internal/core/sync`.

## Interfaces

Core components depend on minimal interfaces implemented by `sb.Client`:
- Story lookup: `GetStoriesBySlug(ctx, spaceID, slug)`
- Content fetch: `GetStoryWithContent(ctx, spaceID, id)`
- Raw read/write: `GetStoryRaw`, `CreateStoryRawWithPublish`, `UpdateStoryRawWithPublish`
- UUID update: `UpdateStoryUUID`

This keeps the core decoupled from the UI and testable with lightweight mocks.

## Testing

- Core: unit tests validate planner ordering, folder creation, raw create/update paths, and logging.
- UI: view, navigation, and interaction tests; state and type mapping is simplified by the unified `PreflightItem`.
- Run with `go test ./...` (optionally `-race -cover`). Network access is not required.
