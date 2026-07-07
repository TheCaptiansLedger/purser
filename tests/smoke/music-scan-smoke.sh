#!/usr/bin/env bash
# tests/smoke/music-scan-smoke.sh
#
# Smoke test for the music scan/identifier pipeline.
#
# Discovers real music albums on disk, triggers a ScanAllRoots job, then
# verifies each album appears in the music queue with the correct folder path
# and track count, and that the identifier logged a scan-start and confidence
# decision for each one.
#
# All expected values are derived from the actual files at LOCAL_MUSIC_ROOT —
# no hardcoded album titles, track counts, or signal thresholds.
#
# Usage (from repo root):
#   tests/smoke/music-scan-smoke.sh
#
# Required:    curl, jq
# Recommended: metaflac (FLAC tags) or ffprobe (any format)
# Optional:    docker (to verify container log output)
#
# Overrides:
#   BASE_URL           API base URL         (default: http://localhost:7474/api/v1)
#   LOCAL_MUSIC_ROOT   Local music dir       (default: test-data/music)
#   CONTAINER_MUSIC    Container music path  (default: /media/content/music)
#   COMPOSE_FILE       docker-compose file   (default: ops/compose.yml)
#   SCAN_TIMEOUT       Job wait timeout (s)  (default: 90)

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:7474/api/v1}"
LOCAL_MUSIC_ROOT="${LOCAL_MUSIC_ROOT:-test-data/music}"
LOCAL_MUSIC_ROOT="${LOCAL_MUSIC_ROOT%/}"
CONTAINER_MUSIC="${CONTAINER_MUSIC:-/media/content/music}"
COMPOSE_FILE="${COMPOSE_FILE:-ops/compose.yml}"
SCAN_TIMEOUT="${SCAN_TIMEOUT:-90}"

PASS=0
FAIL=0

log()  { printf '[smoke] %s\n' "$*"; }
ok()   { printf '  ✓ %s\n' "$*";     PASS=$(( PASS + 1 )); }
fail() { printf '  ✗ %s\n' "$*" >&2; FAIL=$(( FAIL + 1 )); }
note() { printf '      %s\n' "$*"; }

for cmd in curl jq; do
    command -v "$cmd" &>/dev/null || { echo "required: $cmd" >&2; exit 1; }
done

# ── Tag extraction ─────────────────────────────────────────────────────────────

extract_tags() {
    local file="$1"
    case "${file##*.}" in
        flac|FLAC)
            if command -v metaflac &>/dev/null; then
                metaflac --export-tags-to=- "$file" 2>/dev/null
                return
            fi
            ;;
    esac
    if command -v ffprobe &>/dev/null; then
        ffprobe -v quiet -print_format flat -show_format "$file" 2>/dev/null \
            | grep 'format\.tags\.' \
            | sed 's/format\.tags\.\([^=]*\)="\(.*\)"/\U\1\E=\2/'
    fi
}

get_tag() {
    # get_tag <raw_tag_string> <KEY>  — case-insensitive
    printf '%s\n' "$1" | grep -i "^$2=" | head -1 | cut -d= -f2-
}

# ── Album discovery ────────────────────────────────────────────────────────────
# Prints: dir|track_count|first_music_file — one line per album directory.
# Runs in a subshell (called via process substitution) so local arrays work.
discover_albums() {
    local root="$1"
    [[ -d "$root" ]] || return 0

    declare -A counts=()
    declare -A firsts=()

    while IFS= read -r f; do
        local d; d=$(dirname "$f")
        counts["$d"]=$(( ${counts["$d"]:-0} + 1 ))
        [[ -z "${firsts[$d]:-}" ]] && firsts["$d"]="$f"
    done < <(find "$root" -mindepth 1 -maxdepth 3 -type f \
        \( -iname "*.flac" -o -iname "*.mp3"  -o -iname "*.m4a" \
           -o -iname "*.aac"  -o -iname "*.ogg"  -o -iname "*.opus" \))

    for d in "${!counts[@]}"; do
        printf '%s|%d|%s\n' "$d" "${counts[$d]}" "${firsts[$d]}"
    done
}

# Convert a local album directory path to its in-container equivalent.
to_container_path() {
    local rel="${1#${LOCAL_MUSIC_ROOT}}"
    rel="${rel#/}"
    printf '%s/%s' "$CONTAINER_MUSIC" "$rel"
}

# ── API helpers ────────────────────────────────────────────────────────────────

api_get() { curl -sf "${BASE_URL}/${1}"; }

trigger_scan() {
    curl -sf -X POST "${BASE_URL}/commands" \
         -H 'Content-Type: application/json' \
         -d '{"name":"ScanAllRoots"}' \
    | jq -r '.id'
}

wait_for_job() {
    local job_id="$1"
    local deadline=$(( SECONDS + SCAN_TIMEOUT ))
    while (( SECONDS < deadline )); do
        local done_at
        done_at=$(api_get "jobs/${job_id}" | jq -r '.completedAt // empty')
        [[ -n "$done_at" ]] && return 0
        sleep 2
    done
    log "ERROR: job ${job_id} did not complete within ${SCAN_TIMEOUT}s"
    return 1
}

# Returns a compact single-line JSON entry for a folderPath (pending then matched).
# Uses first() so duplicate entries from re-scans don't corrupt the output.
find_queue_entry() {
    local container_path="$1"
    for status in pending matched; do
        local entry
        entry=$(api_get "music/queue?status=${status}" \
            | jq -c --arg p "$container_path" 'first(.[] | select(.folderPath == $p)) // empty')
        [[ -n "$entry" ]] && { printf '%s' "$entry"; return 0; }
    done
    return 1
}

# ── Discover ───────────────────────────────────────────────────────────────────

log "discovering albums under ${LOCAL_MUSIC_ROOT}"

declare -A album_tracks=()
declare -A album_first=()
declare -A album_tags=()

while IFS='|' read -r local_dir track_count first_file; do
    album_tracks["$local_dir"]="$track_count"
    album_first["$local_dir"]="$first_file"
    album_tags["$local_dir"]=$(extract_tags "$first_file")
    log "  found: ${local_dir##*/} (${track_count} tracks)"
done < <(discover_albums "$LOCAL_MUSIC_ROOT")

if (( ${#album_tracks[@]} == 0 )); then
    log "no music albums found in ${LOCAL_MUSIC_ROOT} — nothing to test"
    exit 0
fi

# ── Trigger scan ───────────────────────────────────────────────────────────────

SCAN_START=$(date -u +%Y-%m-%dT%H:%M:%SZ)
log "triggering ScanAllRoots at ${SCAN_START}"
JOB_ID=$(trigger_scan)
log "job submitted: ${JOB_ID}"
log "waiting for scan to complete (timeout: ${SCAN_TIMEOUT}s)"
wait_for_job "$JOB_ID"
log "scan complete"

# ── Verify queue entries ───────────────────────────────────────────────────────

log ""
log "── queue verification ────────────────────────────────────────────────────"

for local_dir in "${!album_tracks[@]}"; do
    expected_tracks="${album_tracks[$local_dir]}"
    container_path=$(to_container_path "$local_dir")
    album_name="${local_dir##*/}"
    raw_tags="${album_tags[$local_dir]}"

    log ""
    log "  ${album_name}"

    entry=$(find_queue_entry "$container_path") || {
        fail "${album_name}: no queue entry found (path: ${container_path})"
        continue
    }

    got_status=$(jq -r '.status'      <<< "$entry")
    got_tracks=$(jq -r '.totalTracks' <<< "$entry")
    got_path=$(jq -r   '.folderPath'  <<< "$entry")

    [[ "$got_path" == "$container_path" ]] \
        && ok "${album_name}: folderPath correct" \
        || fail "${album_name}: folderPath — got '${got_path}', want '${container_path}'"

    (( got_tracks == expected_tracks )) \
        && ok "${album_name}: totalTracks=${expected_tracks}" \
        || fail "${album_name}: totalTracks — got ${got_tracks}, want ${expected_tracks}"

    if [[ "$got_status" == "matched" ]]; then
        ok "${album_name}: auto-imported (status=matched)"
        note "confidence: $(jq -r '.candidates[0].overallConfidence // "n/a"' <<< "$entry")"

    elif [[ "$got_status" == "pending" ]]; then
        note "status=pending (below auto-import threshold)"

        cand_count=$(jq '.candidates | length' <<< "$entry")
        if (( cand_count > 0 )); then
            ok "${album_name}: ${cand_count} candidate(s) present"

            top_title=$(jq -r '
                .candidates[0].releaseTitle //
                .candidates[0].releaseGroupTitle //
                "(no title)"' <<< "$entry")
            top_conf=$(jq -r '.candidates[0].overallConfidence' <<< "$entry")
            note "top candidate: '${top_title}' (confidence ${top_conf})"

            sig_count=$(jq '.candidates[0].signals | keys | length' <<< "$entry")
            (( sig_count == 7 )) \
                && ok "${album_name}: all 7 confidence signals present" \
                || fail "${album_name}: signals — got ${sig_count}, want 7"

            # Show signal scores alongside what was on disk for easy comparison.
            barcode_on_disk=$(get_tag "$raw_tags" "UPC")
            [[ -z "$barcode_on_disk" ]] && barcode_on_disk=$(get_tag "$raw_tags" "BARCODE")
            isrc_on_disk=$(get_tag "$raw_tags" "ISRC")

            note "signals:"
            note "  barcode=$(jq -r '.candidates[0].signals.barcode'         <<< "$entry")  (disk UPC: '${barcode_on_disk:-none}')"
            note "  isrc=$(jq -r    '.candidates[0].signals.isrc'            <<< "$entry")     (disk ISRC: '${isrc_on_disk:-none}')"
            note "  rgNameFuzzy=$(jq -r '.candidates[0].signals.rgNameFuzzy' <<< "$entry")"
            note "  trackCount=$(jq -r  '.candidates[0].signals.trackCount'  <<< "$entry")"
        else
            note "(no candidates — MusicBrainz source may not be configured)"
        fi
    fi
done

# ── Container log verification ─────────────────────────────────────────────────

log ""
log "── container log verification ────────────────────────────────────────────"

if ! command -v docker &>/dev/null; then
    log "(docker not in PATH; skipping log checks)"
else
    SCAN_LOGS=$(docker compose -f "${COMPOSE_FILE}" logs --since="${SCAN_START}" app 2>/dev/null || true)

    for local_dir in "${!album_tracks[@]}"; do
        container_path=$(to_container_path "$local_dir")
        album_name="${local_dir##*/}"

        if printf '%s\n' "$SCAN_LOGS" | grep -F 'album scan start' | grep -qF "${container_path}"; then
            ok "${album_name}: 'album scan start' in logs"
            conf_line=$(printf '%s\n' "$SCAN_LOGS" \
                | grep -F 'album confidence' | grep -F "${container_path}" | tail -1 || true)
            [[ -n "$conf_line" ]] && note "$(printf '%s' "$conf_line" | sed 's/.*msg=//')"

        elif printf '%s\n' "$SCAN_LOGS" | grep -F 're-scan shortcut' | grep -qF "${container_path}"; then
            ok "${album_name}: re-scan shortcut logged (already imported)"
        else
            fail "${album_name}: no scan log found — identifier may not have run"
        fi
    done
fi

# ── Summary ────────────────────────────────────────────────────────────────────

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
printf '  PASS: %d   FAIL: %d\n' "$PASS" "$FAIL"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
(( FAIL == 0 ))
