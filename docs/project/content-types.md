# Content Types

User selects content type when adding anything to the library. Content type drives terminology, metadata sources, and hierarchy depth.

| `content_type` | Display | Hierarchy | Leaf name | Person role |
|---|---|---|---|---|
| `movie` | Movie | entry only | Movie | Cast |
| `tv` | TV Show | entry → group (Season) → item (Episode) | Episode | Cast |
| `music` | Music | entry → group (Album) → item (Track) | Track | Artist |
| `adult` | Adult | entry → [group (Series)] → item (Scene) | Scene | Performer |
| `jav` | JAV | entry → [group (Series)] → item (Title) | Title | Actress |

## Kind Values

`kind` values in `library_entries` and their typical parent:

| Kind | Parent | Examples |
|---|---|---|
| `network` | none | HBO, Naughty America, Columbia Records |
| `studio` | `network` | production company, adult site, JAV studio |
| `series` | `studio` or `network` | a TV show, an adult site's named series |
| `artist` | none | Fleetwood Mac (music top-level) |
| `movie` | `studio` | collapsed — no separate leaf item |

**Movies are collapsed**: `kind=movie` in `library_entries` is the movie itself. One `item` record is auto-created as its leaf (holds file/status state). No manual group creation.

## UI Hierarchy Browse → Database Mapping

| User action | DB effect |
|---|---|
| Monitor all Fleetwood Mac | `library_entries[artist].monitored=true, monitor_mode=all` |
| Monitor Rumours album only | artist monitored + `groups[Rumours].monitored=true`, others false |
| Monitor specific tracks | group monitored + cherry-picked `items[track].monitored=true` |
| All videos with Alex Coal | `people[Alex Coal].monitored=true` |
| All episodes of a series | `library_entries[series].monitored=true, monitor_mode=all` |
| Future episodes only | `library_entries[series].monitored=true, monitor_mode=future` |
| Specific episodes | series NOT monitored + cherry-picked `items.monitored=true` |
| All movies with Tom Cruise | `people[Tom Cruise].monitored=true` |
| This specific movie | `library_entries[kind=movie].monitored=true` |
