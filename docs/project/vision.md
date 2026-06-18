# Purser — Project Vision

Purser is a self-hosted metadata manager for the media you care about. It maintains a rich, browsable library enriched from external sources (TMDB, TVDB, MusicBrainz, StashDB, and others) and gives users tools to identify gaps in their collection and fill them. The primary value is clean, deep metadata across every content type; the acquisition pipeline (Phase 2) is a disciplined way to go get what's missing.

Designed to run as a standalone binary, a container, or a Kubernetes workload. No cloud account required.

**Module path:** `github.com/purser-app/purser` (update when repo is published)

## What Purser Does

- Maintains a rich, browsable metadata database for multiple content types
- Enriches metadata from external sources (StashDB, TPDB, TMDB, TVDB, MusicBrainz, etc.)
- Tracks monitored items at every level of the content hierarchy
- Searches indexers via Prowlarr for matching releases
- Dispatches grabs to download clients
- Identifies files by OSHash first, then fallback parser chain
- Renames and organizes files on disk using user-defined Go templates
- Exposes a REST API that drives everything — the UI has no privileged access

## What Purser Does Not Do

- Manage indexers (Prowlarr's job, injected via interface)
- Serve or transcode media
- Replace Stash — it can ingest metadata from Stash but is not a Stash replacement
- Handle user authentication (deferred)

## Development Phases

Build in this order. Do not begin Phase 2 work until Phase 1 is solid.

### Phase 1 — Metadata & Browse

- Domain model + database schema
- Metadata ingestion from external sources (StashDB, TPDB, TMDB, TVDB, MusicBrainz, etc.)
- REST API for browsing: library entries, groups, items, people, tags
- React UI: browse the full hierarchy, see performers/cast, filter by any metadata dimension
- Search within the local database

### Phase 2 — Acquisition & Disk Management

- Indexer integration (Prowlarr adapter)
- Download client integration (qBittorrent, Transmission, etc.)
- Release matching and grab pipeline
- File identification (OSHash-first, then configurable parser chain)
- On-disk organization via user-defined naming templates
- Monitoring: wanted/grabbed/imported state machine
