# External Metadata Sources

Adapters live in `internal/adapters/<source>/`. Each implements the `MetadataSource` port.

## Supported Sources

| Source | Adapter | Content Types | Auth |
|---|---|---|---|
| StashDB | `internal/adapters/stashdb/` | adult | API key |
| TPDB | `internal/adapters/tpdb/` | adult, jav | API key |
| TMDB | `internal/adapters/tmdb/` | movie, tv | API key |
| TVDB | `internal/adapters/tvdb/` | tv | API key |
| MusicBrainz | `internal/adapters/mbz/` | music | none (rate-limited) |

## Notes

Document adapter-specific quirks, gotchas, and field mappings here as they are discovered.

### StashDB

- GraphQL API; queries live in `internal/adapters/stashdb/queries/`
- Performer metadata field mapping documented in `docs/technical/person-metadata-keys.md`
- `FetchEntryContent` fetches all scenes for a studio in paginated batches

### MusicBrainz

- Public API; no auth required but rate limit is 1 request/second
- Use the `Retry-After` header when rate limited
