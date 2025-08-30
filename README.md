# Storyblok Sync TUI

Storyblok Sync is a terminal user interface (TUI) that synchronises Stories and Folders between two Storyblok spaces. It allows users to scan source and target spaces, select content, preview collisions and apply changes.

See also:
- Architecture overview: [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)
- Planning and roadmap details: [docs/PLANNING.md](./docs/PLANNING.md)

## Features

- Scan source and target spaces and list stories with metadata.
- Mark individual stories or entire folders for synchronisation.
- Fuzzy search over name, slug and path.
- Preflight collision check with per-item skip or overwrite decisions.
- Sync engine that creates or updates stories in the target space with progress and error reporting.
- Rescan at any time to refresh space data.
- Prefix filter for listing/selection only (no bulk "starts-with" execution).

## User Flow

1. **Welcome/Auth** – enter a token or load it from `~/.sbrc`.
2. **SpaceSelect** – choose source and target spaces.
3. **Scanning** – fetch stories for both spaces.
4. **BrowseList** – navigate and mark stories.
5. **Preflight** – review collisions and choose actions.
6. **Syncing** – apply the sync plan.
7. **Report** – see which items succeeded or failed.

Interrupts: `r` to rescan, `q` to abort.

## Status

- v1.0: Functional and stable TUI for syncing Stories and Folders between Storyblok spaces. Implemented flow: auth, space selection, scan, browse with search, preflight, sync with retries, and final report. Logging is available via `DEBUG` to `debug.log`.

## TODO (Next Steps)

1. [Robust rate limiting & retries](./docs/PLANNING.md)

   - Detect HTTP 429 and parse `Retry-After` in `sb.Client`.
   - Centralize exponential backoff with jitter; honor context cancel.
   - Add tests for backoff, header parsing, and transient errors.

2. Interactive Diff & Merge View (Preflight)

   - Screen/state: add a Diff view for colliding stories with side‑by‑side source/target.
   - Data: fetch full raw payloads; normalize; ignore read‑only fields; focus on `content` + slugs.
   - Diff engine: recursive JSON diff for maps/arrays; match arrays by `_uid` when present; mark add/remove/modify.
   - UI: collapsible per field, collapse unchanged by default; expand changed; keyboard to pick source/target per field; accept‑all‑left/right, search by path; help overlay.
   - Merge: build merged JSON, validate minimal invariants, store decision for the item, feed merged payload into sync.
   - Tests: diff correctness on maps/arrays, large payload performance (bench/light tests), decision persistence.

3. RichText preview

   - Detect RichText fields (root `type=doc`) in story content.
   - Add preview toggle in Diff and Browse: raw JSON vs rendered preview.
   - Implement minimal renderer (paragraphs, headings, bold/italic, links, lists); truncate long blocks with expand.
   - Sanitize/link handling; no external fetches; keep it fast and safe.
   - Tests with fixtures under `testdata/` for common node types and edge cases.

4. UX improvements

   - Publish state UI: show publish/unpublished badge in lists; in Preflight allow per‑item publish toggle (stories only), defaulting from source + plan policy; persist in plan and respect during sync.
   - Per‑item progress, pause/cancel, clearer error surfacing in Sync view.
   - Persist browse collapse across screens; snapshot tests.

5. Performance & caching

   - Bounded worker pool + token-bucket rate limiter.
   - Reuse `ContentManager` more broadly; add simple metrics.
   - Concurrency tests with deterministic ordering.

6. Security & logging

   - Redact tokens; avoid logging large payloads by default.
   - Structured logs with levels; audit for accidental secrets.

7. CI & releases

   - GitHub Actions: `go fmt/vet/test` + `staticcheck` on PRs.
   - Goreleaser for multi-arch binaries; release notes template.

8. Dry-run mode (low priority)

   - Core: no-op write layer that still produces full reports.
   - UI toggle; clear messaging in Report view.
   - Tests verifying zero write calls and identical plan.

9. Component sync (low priority)

- Mode toggle: switch between Stories and Components in the UI.
- API: extend client to list/get/create/update components; handle groups and display names.
- Browse/search: fuzzy search by name, group, and schema keys; filter by group.
- Collision check: detect name/group conflicts and schema diffs.
- Diff/merge: JSON schema diff with collapse/expand; highlight breaking changes (type, required, enum shrink).
- Dependencies: resolve nested component references; compute sync order; warn on missing dependencies.
- Safety/validation: block breaking changes by default or gate behind confirmation; optional dry‑run validator to check impact on existing stories.
- Backups: export target component schemas before overwrite; store under `testdata/` or timestamped snapshots.
- Tests: fixtures for components and dependency graphs; diff and ordering tests.

## Project Structure

```
storyblok-sync/
├─ cmd/sbsync/            # entry point
├─ internal/
│  ├─ ui/                 # Bubble Tea models, views and keybinds
│  ├─ sb/                 # Storyblok API client
│  ├─ config/             # token/config loading and saving
│  └─ core/
│     └─ sync/            # domain sync core (planner/orchestrator/syncer)
└─ testdata/              # JSON fixtures
```

The sync core has been extracted to `internal/core/sync`. Future modules (`infra`) will house infrastructure helpers described in the roadmap. For a deeper dive into responsibilities and data flow, see [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md).

## Contributing

See [AGENTS.md](AGENTS.md) for coding conventions and testing requirements. Submit pull requests with a short description of the change and note any deviations from the guidelines.
