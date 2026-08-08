#!/usr/bin/env bash
# Regenerates this directory's three tagged FLAC fixtures. Not run by any
# Makefile target — the generated .flac files are committed (binary
# fixtures, same convention as internal/adapters/pipeline/music/testdata/)
# so `make k6-ci` never depends on ffmpeg being installed on the CI runner.
# Run this by hand only when the fixture data itself needs to change —
# keep it in sync with internal/adapters/musicbrainz/fixtureserver's MBID/
# title constants and test/k6/flow/accept_candidate_test.js's expectations.
#
# 01/02 (ReleaseMBID, direct-ID auto-import path) carry MUSICBRAINZ_ALBUMID
# so identify resolves them with zero search calls. 03 (ReleaseMBID2,
# manual AcceptCandidate path) deliberately carries no MusicBrainz tags at
# all and an ALBUM/ARTIST that won't fuzzy-match anything (the fixture
# server's search routes always return empty) — identify finds nothing,
# decide never persists, and the group is only ever resolved by a human
# (the k6 flow) supplying ReleaseMBID2 directly.
#
# Tag key choice matches internal/adapters/pipeline/music/fingerprinter.go's
# canonicalTagAliases exactly (FLAC/Vorbis-comment normalizes ALBUMARTIST/
# DISCNUMBER/TRACKNUMBER to lowercase album_artist/disc/track; ALBUM/TITLE/
# ISRC stay as written) — verified via ffprobe after generation, not
# assumed. -compression_level 0 and a 6-second sine tone (not silence,
# which FLAC would compress away almost entirely) keep both files safely
# over filehash.OSHash's 64 KiB minimum. MUSICBRAINZ_ALBUMID (the release
# MBID) is what M7's direct-ID short-circuit
# (internal/adapters/pipeline/music/identifier.go's directIDFromRelease)
# actually keys on — set to fixtureserver.ReleaseMBID so identify resolves
# straight to LookupRelease with no fuzzy-search route needed on the
# fixture server at all.
set -euo pipefail
cd "$(dirname "$0")"

ffmpeg -y -f lavfi -i "sine=frequency=440:duration=6" -ar 44100 -ac 2 -compression_level 0 \
  -metadata ALBUM="K6 Fixture Album" -metadata ALBUMARTIST="K6 Fixture Artist" \
  -metadata TITLE="K6 Fixture Track One" -metadata TRACKNUMBER="1" -metadata DISCNUMBER="1" \
  -metadata ISRC="XXK6F0000001" \
  -metadata MUSICBRAINZ_ALBUMID="33333333-3333-3333-3333-333333333333" \
  -metadata MUSICBRAINZ_RELEASEGROUPID="22222222-2222-2222-2222-222222222222" \
  01-k6-fixture-track-one.flac

ffmpeg -y -f lavfi -i "sine=frequency=523:duration=6" -ar 44100 -ac 2 -compression_level 0 \
  -metadata ALBUM="K6 Fixture Album" -metadata ALBUMARTIST="K6 Fixture Artist" \
  -metadata TITLE="K6 Fixture Track Two" -metadata TRACKNUMBER="2" -metadata DISCNUMBER="1" \
  -metadata ISRC="XXK6F0000002" \
  -metadata MUSICBRAINZ_ALBUMID="33333333-3333-3333-3333-333333333333" \
  -metadata MUSICBRAINZ_RELEASEGROUPID="22222222-2222-2222-2222-222222222222" \
  02-k6-fixture-track-two.flac

mkdir -p ambiguous
ffmpeg -y -f lavfi -i "sine=frequency=659:duration=6" -ar 44100 -ac 2 -compression_level 0 \
  -metadata ALBUM="K6 Ambiguous Album" -metadata ALBUMARTIST="K6 Ambiguous Artist" \
  -metadata TITLE="K6 Ambiguous Track" -metadata TRACKNUMBER="1" -metadata DISCNUMBER="1" \
  ambiguous/03-k6-ambiguous-track.flac

# organize/ — test/k6/flow/organize_music_test.js's own isolated copy of
# the direct-ID pair. Same MBID/tags as 01/02 (so it identifies and
# auto-imports exactly the same way) but genuinely distinct audio content
# (different sine frequencies), never the SAME bytes as 01/02 — that
# flow calls OrganizerService.Organize, which physically moves the file;
# reusing 01/02's own bytes (even from a different path — filehash's
# "already known" short-circuit matches on content hash, not path) would
# let a hash-based match land the SAME MediaFile row on two different
# physical locations depending on scan order, and moving 01/02 out of
# .cidata/scan-music/ would silently break accept_candidate_test*.js's own
# rerun against those exact files later in the same k6-ci invocation. See
# that flow's own header comment.
mkdir -p organize
ffmpeg -y -f lavfi -i "sine=frequency=349:duration=6" -ar 44100 -ac 2 -compression_level 0 \
  -metadata ALBUM="K6 Fixture Album" -metadata ALBUMARTIST="K6 Fixture Artist" \
  -metadata TITLE="K6 Fixture Track One" -metadata TRACKNUMBER="1" -metadata DISCNUMBER="1" \
  -metadata ISRC="XXK6F0000001" \
  -metadata MUSICBRAINZ_ALBUMID="33333333-3333-3333-3333-333333333333" \
  -metadata MUSICBRAINZ_RELEASEGROUPID="22222222-2222-2222-2222-222222222222" \
  organize/01-k6-organize-track-one.flac

ffmpeg -y -f lavfi -i "sine=frequency=392:duration=6" -ar 44100 -ac 2 -compression_level 0 \
  -metadata ALBUM="K6 Fixture Album" -metadata ALBUMARTIST="K6 Fixture Artist" \
  -metadata TITLE="K6 Fixture Track Two" -metadata TRACKNUMBER="2" -metadata DISCNUMBER="1" \
  -metadata ISRC="XXK6F0000002" \
  -metadata MUSICBRAINZ_ALBUMID="33333333-3333-3333-3333-333333333333" \
  -metadata MUSICBRAINZ_RELEASEGROUPID="22222222-2222-2222-2222-222222222222" \
  organize/02-k6-organize-track-two.flac
