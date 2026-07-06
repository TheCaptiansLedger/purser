# Music Organization

> This document is the canonical reference for how Purser models, identifies, and imports music. It covers the domain model, the full import pipeline, confidence scoring, artist handling, and worked examples. Read this before touching any music-related code.

---

## Domain Model

Music is organized across four levels. Each level is strictly hierarchical.

```
Artist
  └── Release Group  (UI label: "Album")
        └── Release  (a specific edition)
              └── Track
```

### Artist

The performing entity — a band, a duo, or a solo performer.

- **Canonical name**: taken directly from MusicBrainz `name` field. No local string manipulation. "The Band" stays "The Band"; "Bob Dylan" stays "Bob Dylan". MBZ is the authority.
- **Aliases**: all known alternate names from MBZ, stored and searchable.
- **Images**: artist logo/banner fetched from TheAudioDB (linked via MBID) and/or Fanart.tv.
- **Metadata**: founded date, founded city/area, dissolved date, ISNI code, bio.
- **Tags/genres**: imported from MBZ.
- **Social links**: official homepage, Last.fm page, and other links from MBZ relationships.
- **Members**: for a band, the current and past members are imported as People records and associated with the Artist with their roles (guitarist, vocalist, etc.). If the role is unknown, record as "member of". See [Solo Artist Handling](#solo-artist-handling) below.

### Release Group (UI: "Album")

A logical work — the conceptual album independent of its editions.

- **Type**: Studio, Live, Compilation, Single/EP. Drives which discography chip it appears under in the UI.
- **Belongs to**: one Artist.
- **Contains**: one or more Releases.
- When a Release Group is first created, Purser immediately fetches **all known Releases** for that RG from MBZ and creates stub records for every one — including editions the user does not yet have files for. This is what allows the user to track "I have this pressing but not that one."

### Release

A specific physical or digital edition of a Release Group.

- **Fields**: title, release country, release date, label, catalog number, barcode, format (CD/Digital/Vinyl/etc.), medium count.
- **Belongs to**: one Release Group.
- **Contains**: one or more Tracks, organized across one or more media.
- This is what the user actually has on disk.

### Track

An individual audio file belonging to a Release.

- **Computed identifiers**: md5, sha1, oshash, acoustid, ISRC (from tags), MBZ Recording ID.
- **All metadata is user-correctable**: a user can update any track's artist, Release Group, and Release after import in case identification was wrong.
- Users cannot import individual tracks; they import Releases. But track-level identifiers are stored to help identify which Release a track actually belongs to.

---

## Monitoring

Monitoring is set at the **Release Group** level.

- By default, monitoring a Release Group means monitoring the **MBZ canonical release** for that RG — the one MBZ marks as the default. This is what the casual collector expects.
- If the user wants to monitor additional Releases (a specific pressing, a remaster, a deluxe edition), they opt in explicitly via the UI.
- Individual tracks are not monitored.

---

## Solo Artist Handling

A solo artist is a Person first, then an Artist.

When importing a solo artist (e.g., Stevie Nicks):

1. Create a **Person** record with biography fields: birth name, birth date, birth place, death date if applicable — all from MBZ.
2. Create an **Artist** record with the same canonical name, linked to that Person. The Person *is* the Artist.
3. Do not import "members" — the solo artist has no band members. The MBZ `member-of` relationship (e.g., Stevie Nicks → Fleetwood Mac) is the reverse direction and is resolved separately: when the user imports Fleetwood Mac, the already-existing Person "Stevie Nicks" is linked in as a member.

For a band (e.g., REO Speedwagon):

1. Create an **Artist** record for the band.
2. Fetch all MBZ member relationships (current and past).
3. For each member: create a **Person** record if one does not already exist, then associate the Person with the Artist and record their role. If the role is ambiguous or unknown, record "member of."

---

## Disk Layout and Multi-Disc Grouping

Purser follows the same folder-grouping logic as Lidarr.

- Each distinct parent folder under the music root is treated as one Release candidate.
- Sub-folders named `CD1`, `CD2`, `Disc 1`, `Disc 2`, etc. are grouped together as multiple media of the **same Release**. They are not treated as separate albums.
- Separate albums are distinct independent parent folders.
- When multi-disc grouping fires, all files across all disc sub-folders are combined into a single identification pass with a total track count that spans all media.

MBZ embedded tags take priority over folder structure for verification. If the track tags carry MBZ Recording IDs that confirm a 2-CD release, that confirmation stands even if the folder layout is ambiguous.

---

## Import Pipeline

### Step 1 — Scan

Walk the music root. Group files by folder. Apply multi-disc grouping. Each group becomes one Release candidate for the identification pipeline.

### Step 2 — Tag Extraction

Read embedded tags from every file in the group:

- Album artist, track artist(s)
- Album title, track titles
- Track number, total tracks, disc number, total discs
- Date/year
- Label, catalog number, barcode/UPC
- ISRC per track
- Any existing MBZ IDs (Release ID, Recording IDs)

### Step 3 — Re-scan Shortcut

If a MBZ Release ID is already written into the file tags (from a previous import), skip the identification pipeline entirely. Look the Release up directly in MBZ, verify the track count matches, and proceed to persist/update. This is how re-scans after file renames or moves are handled cheaply.

### Step 4 — Artist Resolution

1. Read the album artist tag. Do not manipulate the string.
2. Search the local database for a matching Artist.
3. If not found: search MBZ by name. Use the MBZ `name` field as the canonical display name. Use the `sort-name` field only for sort ordering, never for display.
4. On first creation of an Artist, import all associated metadata: bio, images, founded/dissolved/area/ISNI, tags, aliases, social links, and members (see [Solo Artist Handling](#solo-artist-handling)).

### Step 5 — Release Group Identification

Identification runs as a **cascade**. Each stage produces a confidence score for that signal. The pipeline stops as soon as overall confidence crosses the auto-import threshold; remaining signals are skipped. All scores are recorded regardless, so the user can see them in the import queue.

**Signal cascade (highest to lowest confidence):**

1. **Barcode/UPC** — if the file tags carry a barcode, query MBZ barcode search. A match directly returns the Release and its Release Group. Confidence: maximum. Skip remaining signals.
2. **ISRC** — each ISRC maps to a MBZ Recording, which belongs to a Release, which belongs to a Release Group. If the ISRCs in the file tags all agree on a single Release Group, confidence is very high. Skip remaining signals.
3. **Fuzzy name + track count** — strip remaster/year/format suffixes from the album tag (e.g., "Hi Infidelity (2024 Remaster)" → "Hi Infidelity"). Fuzzy-match against all Release Group names for the identified Artist. Candidates must also match the total track count extracted from tags or file count.
4. **Track title set match** — if two or more Release Groups survive name+count (e.g., a studio album and a live album with the same track count and similar name), compare the full track title list. Exact title overlap selects the winner.
5. **Duration match** — sum of track durations within a tolerance window, or per-track duration comparison. Distinguishes the studio release from live versions recorded at the same session, or vinyl-speed transfers from digital masters.
6. **AcoustID** — compute acoustic fingerprints and query the AcoustID service. Applied per-track. Partial matches contribute to the score. This is the last resort before the import queue.

### Step 6 — Release Identification

Once the Release Group is known, identify the specific Release (edition):

1. Fetch all Releases for the Release Group from MBZ.
2. **Eliminate** any Release whose track count does not match.
3. **Barcode** — if the UPC tag matches a Release's barcode in MBZ, that Release is identified. Done.
4. **Year** — if only one Release matches the year in the tags, it is a strong candidate.
5. **Per-track duration match** — compare each file's audio duration against each candidate Release's MBZ tracklist within a ±2s window. Score = fraction of tracks that match. Scores across candidates are compared; the highest-scoring candidate wins if it is sufficiently ahead of the next.
6. **AcoustID** — per-track fingerprint comparison against MBZ Recordings. Each track that matches contributes to the confidence score. A partial match (e.g., 9 of 10 tracks match) is valid and still scores highly.

### Step 7 — Confidence Decision

Each signal produces its own score. These are surfaced individually in the import queue. The overall confidence is a weighted combination.

| Signal | Weight |
|---|---|
| Barcode match | 1.0 (auto-import alone) |
| ISRC consensus | 0.95 |
| RG name (exact) | 0.60 |
| RG name (fuzzy) | 0.40 |
| Track count | 0.20 |
| Track title set | 0.25 |
| Duration match | 0.30 |
| AcoustID | 0.35 |

If overall confidence exceeds the auto-import threshold: proceed directly to Step 8.

If below threshold or ambiguous: send to the **import queue**. Files are grouped by folder. The queue card shows every signal and its individual score, so the user can see exactly what fired and why it was not enough. The user can search MBZ for an artist, pick a Release Group, and pick a Release manually — even one that does not actually match what the system found.

### Step 8 — Persist

Create or update:

- **Artist** (if not already present, with full metadata enrichment)
- **Release Group** (if not already present; also create stubs for all known Releases of that RG)
- **Release** (title, country, date, label, catalog number, barcode, format)
- **Track** records — one per file, with md5, sha1, oshash, ISRC, MBZ Recording ID. AcoustID computation is queued as an async job if not already computed.

### Step 9 — Write IDs Back to Tags

After a successful import, write the MBZ Release ID and per-track MBZ Recording IDs back into the file tags. This is the re-scan resilience mechanism. When the scanner encounters these files again after a rename or move, Step 3 short-circuits the entire identification pipeline.

---

## Import Queue

The queue groups candidates by source folder. Each queue entry shows:

```
Folder:           REO Speedwagon - Hi Infidelity (2024) [FLAC 24-192]/
Tracks:           10 across 1 disc

Identified:
  Artist:           REO Speedwagon           100%  (MBZ exact match)
  Release Group:    Hi Infidelity            100%  (barcode hit)
  Release:          Hi Infidelity (2024)     100%  (barcode hit)

Signal breakdown:
  Barcode:          074646161425 → matched    ✓
  ISRC:             not evaluated
  RG name fuzzy:    not evaluated
  Track count:      not evaluated
  Duration:         not evaluated
  AcoustID:         not evaluated

Overall confidence: 100% — AUTO-IMPORT
```

For a below-threshold entry the breakdown shows which signals fired and at what strength, so the user understands the ambiguity before manually selecting.

---

## Worked Examples

### REO Speedwagon — Hi Infidelity (2024 Remaster)

**Files on disk:**

```
REO Speedwagon - Hi Infidelity (2024) [FLAC 24-192]/
  01 Don't Let Him Go.flac
  02 Keep on Loving You.flac
  ...
  10 I Wish You Were There.flac
  Cover.jpg
```

**Tags extracted:**

| Tag | Value |
|---|---|
| ARTIST | REO Speedwagon |
| ALBUM | Hi Infidelity (2024 Remaster) |
| TRACKTOTAL | 10 |
| DISCTOTAL | 1 |
| DATE | 1980-11-21 |
| LABEL | Epic - Legacy |
| UPC | 0074646161425 |
| ISRC (track 1) | USSM10012807 |
| MBZ IDs | none |

No MBZ IDs in tags — re-scan shortcut does not fire.

**Artist resolution:** "REO Speedwagon" not in local DB. MBZ search → MBID `bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1`. Artist is a band. On create:

- Bio imported
- Images from TheAudioDB (linked via MBID)
- Founded 1967, Champaign, Illinois
- ISNI imported
- Aliases imported
- Members: Kevin Cronin (vocals), Gary Richrath (guitar), Neal Doughty (keyboards), Bruce Hall (bass), Alan Gratzer (drums), Dave Amato (guitar), et al. — each created as a Person and associated with the Artist with their role
- Tags: Rock, Hard Rock, Pop/Rock, Arena Rock
- Social links: official homepage, Last.fm page, etc.

**Release Group identification:** UPC `0074646161425` present in tags. MBZ barcode search returns Release `1e639bf3-6b4c-4e1a-9d15-c61511804c8f`. Release Group derived from that Release: "Hi Infidelity", type Studio. Confidence: maximum. ISRC and all fuzzy signals skipped.

**Release identification:** Already resolved by barcode — `1e639bf3`. Title "Hi Infidelity", 10 tracks, Digital Media, 2024, barcode `074646161425`, label Epic-Legacy.

**Persist:** Artist REO Speedwagon + all members as People. Release Group "Hi Infidelity" (Studio) + stubs created for all other known editions of Hi Infidelity from MBZ. Release `1e639bf3` with full edition metadata. 10 Track records with md5, sha1, oshash, ISRC per track. AcoustID queued async.

**Write IDs back:** MBZ Release ID written to all 10 FLAC files. MBZ Recording IDs written per track.

**Decision:** 100% confidence → auto-import, no queue.

**UI:** Artist page for REO Speedwagon shows "Hi Infidelity" under the Studio Albums chip in the Discography section. Clicking the album shows all known editions (stubs), with the 2024 Digital remaster marked as imported.

---

### Stevie Nicks — The Enchanted Works of Stevie Nicks (worst case: no barcode, no ISRCs, no MBZ IDs)

**Files on disk:**

```
Stevie Nicks - The Enchanted Works of Stevie Nicks (1998)/
  CD1/
    01 Enchanted.flac
    ...
    15 Bella Donna.flac
  CD2/
    01 Edge of Seventeen (live).flac
    ...
    15 Has Anyone Ever Written Anything for You?.flac
  CD3/
    01 Twisted (demo).flac
    ...
    16 Rhiannon (piano version).flac
```

**Tags extracted:** `ARTIST: Stevie Nicks`, `ALBUM: The Enchanted Works of Stevie Nicks`, `DATE: 1998`, track titles present, `DISCTOTAL: 3`. No barcode, no ISRCs, no MBZ IDs.

**Scan / multi-disc grouping:** `CD1`, `CD2`, `CD3` sub-folders detected. Grouped as a single Release candidate: 46 tracks across 3 media.

**Re-scan shortcut:** No MBZ IDs — does not fire.

**Artist resolution:** "Stevie Nicks" not in local DB. MBZ search → MBID `b7f2cca2-72c6-41fb-ae33-53370fc62fe7`. MBZ `name` = "Stevie Nicks" (canonical display name). This is a solo artist.

Solo artist handling:
- Create **Person**: Stevie Nicks, born May 26, 1948, Phoenix AZ (from MBZ)
- Create **Artist**: "Stevie Nicks", linked to that Person
- No member import — the Person *is* the Artist
- Import bio, images (TheAudioDB/Fanart.tv), tags, aliases, social links

Note on Fleetwood Mac: MBZ shows Stevie Nicks is a member of Fleetwood Mac. This relationship is not processed here. It resolves in the opposite direction when Fleetwood Mac is imported and the already-existing Person "Stevie Nicks" is linked as a member.

**Release Group identification — no barcode, no ISRCs:**

Barcode signal: not available. ISRC signal: not available.

Fuzzy name match: fetch all Release Groups for MBID `b7f2cca2` from MBZ. Match album tag "The Enchanted Works of Stevie Nicks" against every RG name. Result: exact match on "The Enchanted Works of Stevie Nicks". No ambiguity — only one RG has this name in her catalog.

Track count cross-check: does this RG have a Release with 46 tracks across 3 discs? Yes → confirmed.

Track title set match: compare the 46 track titles extracted from tags against the MBZ Release tracklist. All 46 titles match. No competing RG candidate survives.

Signal breakdown at this point:

```
Barcode:           not available
ISRC:              not available
RG name (exact):   100%
Track count:       100%  (46/46)
Track title set:   100%  (46/46 titles match)
Duration:          not yet evaluated
AcoustID:          not yet evaluated
```

Combined confidence already exceeds auto-import threshold. Duration and AcoustID not needed.

**Release identification — no barcode:**

Get all Releases for "The Enchanted Works of Stevie Nicks" from MBZ. Likely 2–3 candidates (original US, possible European or international variants). Eliminate any that do not have 46 tracks across 3 discs. Read audio duration from each file (from the bitstream, independent of tags). Compare per-track duration against each candidate's MBZ tracklist within ±2s.

If the original US CD pressing and a remaster have measurably different track durations (typical for remasters), duration scoring separates them. If one candidate scores 44/46 tracks within ±2s and the others score lower, that candidate is selected.

If after duration matching two candidates remain within close confidence, compute AcoustID on the differing tracks only and query the AcoustID service. AcoustID resolves it.

**Persist:** Person Stevie Nicks. Artist "Stevie Nicks" linked to Person. Release Group "The Enchanted Works of Stevie Nicks" (type: Compilation) + stubs for all known editions. Release with 3-disc, 1998, US, Atlantic, catalog `83093-2`. 46 Track records with md5, sha1, oshash. ISRCs queued from MBZ Recording lookups. AcoustID queued async.

Tracks with guest artist credits ("Stop Draggin' My Heart Around" — Stevie Nicks & Tom Petty and the Heartbreakers; "Gold" — John Stewart feat. Stevie Nicks; "Whenever I Call You Friend" — Kenny Loggins & Stevie Nicks): guest artists created as People/Artists if not already present and linked as featured credits on those specific tracks. They are not members of Stevie Nicks.

**Write IDs back:** MBZ Release ID and per-track Recording IDs written into all 46 FLAC files.

**Decision:** Combined confidence exceeds threshold → auto-import, no queue.

**UI:** Artist page for Stevie Nicks shows "The Enchanted Works of Stevie Nicks" under the **Compilation** chip in the Discography section, not under Studio Albums.
