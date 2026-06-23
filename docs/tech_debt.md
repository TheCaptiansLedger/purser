# Tech Debt

Items explicitly deferred from active epics. Each entry names the originating discussion, the gap, and what would be needed to close it.

---

## Group-Level Contributor Credits (`group_people`)

**Origin:** Tagging Refactor epic — domain model design session.

**Gap:** `item_people` links people to individual tracks/scenes/episodes with a role. `entry_people` links people to artist library entries (band members). Neither covers the case where a person contributed to an entire album as a unit — e.g. Jimmy Iovine as producer of *Bella Donna*, or a film composer credited on a whole soundtrack album.

Today the closest approximations are:
- Link the person to every individual track via `item_people` (role=producer) — semantically correct but operationally tedious for a 12-track album
- Link the person to the artist library entry via `entry_people` — semantically wrong (it says they are part of the artist, not that they worked on one album)

**What is needed:** A `group_people` join table mirroring `item_people` at the group level:

```sql
CREATE TABLE group_people (
    group_id  TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT NOT NULL,
    PRIMARY KEY (group_id, person_id, role)
);
```

Domain: add `People []ItemPerson` to `domain.Group`. Repository: load/save group people. API: expose on group detail response. UI: show in group detail panel alongside track list.

**Priority:** Medium. Track-level credits are sufficient for Phase 1 browsing. This becomes important when MusicBrainz relationship data (producers, engineers, artwork) is imported.

---

## Book Series Hierarchy

**Origin:** Tagging Refactor epic — domain model design session.

**Gap:** No modeled concept for book series (Discworld, Harry Potter). The `groups` table is the natural fit (same role as music albums between artist and tracks), but books currently use a collapsed `KindBook` library entry with no intermediate group layer.

**Options to evaluate when building the Books module:**
1. `library_entry(kind=author) → group(series) → item(book-file)` — consistent with music pattern; series as a group
2. `library_entry(kind=author) → library_entry(kind=series) → library_entry(kind=book)` — three levels of entries, no groups involved
3. Treat series as a tag (`key=series, value=Discworld`) and keep the flat `author → book` tree — loses ordering/sequencing

Option 1 is preferred for consistency. Books are currently defined as collapsed entries (`KindBook`), which is incompatible with the music pattern — this will require a design decision before the Books module is built.

**Priority:** Low. Books module not yet started.

---

## Default Naming Templates Per Content Type

**Origin:** Tagging Refactor epic — domain model design session.

**Gap:** `naming_configs` table exists with `folder_template` and `file_template` per content type, but no default templates are shipped. Users must configure templates manually before the acquisition pipeline can rename or organize files.

**What is needed:** Sensible defaults baked into the initial migration or seeded on first run:

| Content Type | Default Folder Template | Default File Template |
|---|---|---|
| movie | `{studio}/{title} ({year})` | `{title} ({year})` |
| tv | `{series}/Season {season_number}` | `S{season:02d}E{episode:02d} - {title}` |
| music | `{artist}/{album} ({year})` | `{track:02d} - {title}` |
| adult | `{network}/{studio}` | `{title}` |
| book | `{author}/{title}` | `{title}` |

Templates should be documented and the token set for each content type should be defined before implementation.

**Priority:** Low. Needed before Phase 2 (acquisition pipeline) is meaningful.

---

## People-Level Tags

**Origin:** Tagging Refactor epic — domain model design session.

**Decision:** Explicitly deferred as not needed. People are associated with content via `item_people` (with role) and `entry_people` (band membership). Tagging a person (e.g., `genre=Action` on an actor) adds complexity without clear value — filter by the content they appear in instead.

**Reopen if:** A concrete use case emerges that cannot be served by filtering items/entries by the person's involvement.
