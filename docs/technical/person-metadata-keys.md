# Person Metadata Keys

`Person.Metadata` is a `map[string]any` that holds source-specific enrichment
fields. Keys are normalized across sources so the UI can render them without
source-specific logic — the adapter is responsible for the mapping, not the
display layer.

Unknown keys in `metadata` are silently ignored by the UI. Adapters must only
set keys listed here; non-standard keys will not be rendered.

---

## Universal Keys

Available from any source that provides them. Use these key names regardless of
what the upstream source calls the field.

| Key | Type | Format | Notes |
|-----|------|--------|-------|
| `birthdate` | string | `YYYY-MM-DD` | TMDB calls this `birthday`; normalize on import |
| `gender` | string | `"male"` \| `"female"` \| `"non_binary"` \| `"transgender_male"` \| `"transgender_female"` \| `"intersex"` \| `"unknown"` | Normalize all source enums to these values |
| `nationality` | string | ISO 3166-1 alpha-2 (e.g. `"US"`, `"JP"`) or plain country name | Use ISO code when the source provides it |
| `homepage` | string | URL | Official personal or professional website |
| `disambiguation` | string | plain text | Parenthetical string to distinguish same-name entities (StashDB, MusicBrainz) |

---

## Performer Keys (Adult / JAV — StashDB)

| Key | Type | Format | StashDB field | Status |
|-----|------|--------|---------------|--------|
| `height` | string | `"X cm"` | `height` (int, cm) | implemented |
| `weight` | string | `"X kg"` | N/A — not in StashDB schema | not available |
| `hair_color` | string | lowercase (e.g. `"brunette"`) | `hair_color` | implemented |
| `eye_color` | string | lowercase (e.g. `"brown"`) | `eye_color` | implemented |
| `measurements` | string | `"band-waist-hip"` (e.g. `"32-24-36"`) | `measurements.band_size` + `waist` + `hip` | implemented |
| `cup_size` | string | letter (e.g. `"C"`) | `measurements.cup_size` | implemented |
| `tattoos` | string | description text | `tattoos` | implemented |
| `piercings` | string | description text | `piercings` | implemented |
| `career_start` | string | `"YYYY"` | `career_start_year` | implemented |
| `career_end` | string | `"YYYY"` | `career_end_year` | **missing** |
| `ethnicity` | string | e.g. `"Caucasian"`, `"Asian"`, `"Latina"` | `ethnicity` | **missing** |
| `breast_type` | string | `"Natural"` \| `"Fake"` | `breast_type` (enum: NATURAL→"Natural", FAKE→"Fake", NA→omit) | implemented |
| `gender` | string | see Universal keys | `gender` (enum) | **missing** |
| `nationality` | string | ISO alpha-2 | `country` | **missing** |
| `disambiguation` | string | plain text | `disambiguation` | **missing** |

### Display Groups (UI rendering order)

**Physical**
: `height`, `weight`, `measurements`, `cup_size`, `hair_color`, `eye_color`

**Background**
: `birthdate`, `nationality` (with flag), `ethnicity`, `gender`

**Career**
: `career_start`, `career_end`, `breast_type`

**Notes**
: `tattoos`, `piercings`, `disambiguation`

---

## Actor / Director Keys (Movie / TV — TMDB)

| Key | Type | Format | TMDB field | Notes |
|-----|------|--------|------------|-------|
| `biography` | string | long-form text | `biography` | May be empty for minor credits |
| `birthdate` | string | `YYYY-MM-DD` | `birthday` | Normalize key name on import |
| `deathday` | string | `YYYY-MM-DD` | `deathday` | Only set if person is deceased |
| `place_of_birth` | string | plain text (city, country) | `place_of_birth` | |
| `gender` | string | see Universal keys | `gender` (int: 0=unknown, 1=female, 2=male, 3=non_binary) | Normalize int to string on import |
| `known_for` | string | e.g. `"Acting"`, `"Directing"`, `"Production"` | `known_for_department` | |
| `imdb_id` | string | `"nmXXXXXXX"` | `imdb_id` | Cross-reference only; not displayed |
| `homepage` | string | URL | `homepage` | |

### Display Groups (UI rendering order)

**Background**
: `birthdate`, `deathday`, `place_of_birth`, `nationality`, `gender`

**Career**
: `known_for`

**Bio**
: `biography`

---

## Artist / Musician Keys (Music — MusicBrainz)

| Key | Type | Format | MusicBrainz field | Notes |
|-----|------|--------|-------------------|-------|
| `type` | string | `"Person"` \| `"Group"` \| `"Orchestra"` \| `"Choir"` \| `"Character"` \| `"Other"` | `type` | Determines whether Members section is shown |
| `gender` | string | see Universal keys | `gender` | Only relevant when `type=Person` |
| `area` | string | plain text | `area.name` | Main associated country or region |
| `begin_area` | string | plain text | `begin-area.name` | Birthplace (Person) or formation city (Group) |
| `end_area` | string | plain text | `end-area.name` | Deathplace or dissolution city; omit if not ended |
| `begin_date` | string | `YYYY`, `YYYY-MM`, or `YYYY-MM-DD` | `life-span.begin` | Birth or formation date |
| `end_date` | string | `YYYY`, `YYYY-MM`, or `YYYY-MM-DD` | `life-span.end` | Death or disbandment date |
| `ended` | boolean | `true` \| `false` | `life-span.ended` | Whether the artist/group is dissolved/deceased |
| `disambiguation` | string | plain text | `disambiguation` | e.g. `"the rap group"` for common names |
| `ipi` | string | 11-digit code | `ipis[0]` | Interested Parties Information; royalties identifier |

### Display Groups (UI rendering order)

**Background**
: `type`, `gender`, `area`, `begin_area`, `end_area`

**Dates**
: `begin_date`, `end_date`, `ended`

**Identifiers**
: `ipi`, `disambiguation`

---

## Implementation Notes

- Adapters must normalize values to the formats above before storing them in
  `Metadata`. The UI makes no attempt to parse raw source values.
- The `nationality` field should always be an ISO 3166-1 alpha-2 code when the
  source provides a machine-readable country. The `CountryChip` UI component
  converts it to a flag emoji + country name.
- Keys not listed in this file should not be stored in `Person.Metadata`. If a
  source provides a useful field not listed here, add it to this document first.
- `biography` / long-form text already has a dedicated `Person.Overview` field at
  the domain level. Only store `biography` in `Metadata` if the source provides
  it separately from the overview (e.g. TMDB provides structured bio separate from
  any overview). Prefer `Person.Overview` for the primary bio text.
