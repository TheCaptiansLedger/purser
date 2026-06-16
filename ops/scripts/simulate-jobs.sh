#!/usr/bin/env bash
# simulate-jobs.sh — submit fake background jobs to test the UI job system.
#
# Usage:
#   ./simulate-jobs.sh [count] [base-url]
#
# Examples:
#   ./simulate-jobs.sh             # 6 jobs against http://localhost:7474
#   ./simulate-jobs.sh 10
#   ./simulate-jobs.sh 4 http://localhost:9090

set -euo pipefail

BASE="${2:-${PURSER_URL:-http://localhost:7474}}"
COUNT="${1:-6}"
ENDPOINT="$BASE/api/v1/jobs/_simulate"

NAMES=(
  "RefreshStudio"
  "ScanLibrary"
  "FetchMetadata"
  "ImportPerformers"
  "RebuildIndex"
  "SyncExternalIDs"
  "DownloadCovers"
  "UpdateTags"
)

# log-uniform random integer in [lo_ms, hi_ms]
rand_ms() {
  local lo=$1 hi=$2
  # awk PRNG seeded from /dev/urandom word
  local seed=$(od -An -N4 -tu4 /dev/urandom | tr -d ' ')
  awk -v seed="$seed" -v lo="$lo" -v hi="$hi" '
    BEGIN {
      srand(seed)
      lo_log = log(lo); hi_log = log(hi)
      print int(exp(lo_log + rand() * (hi_log - lo_log)) + 0.5)
    }'
}

rand_steps() {
  local seed=$(od -An -N4 -tu4 /dev/urandom | tr -d ' ')
  awk -v seed="$seed" '
    BEGIN {
      srand(seed)
      r = rand()
      if (r < 0.25) { print 0 }
      else           { print int(5 + rand() * 15 + 0.5) }
    }'
}

extract_id() {
  # pull the first "id":"<value>" from the JSON line
  grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"//;s/"//'
}

echo "Purser job simulator"
echo "  target : $ENDPOINT"
echo "  count  : $COUNT"
echo ""

if ! curl -sf "$BASE/api/v1/config" > /dev/null 2>&1; then
  echo "ERROR: cannot reach $BASE — is Purser running?" >&2
  exit 1
fi

for i in $(seq 1 "$COUNT"); do
  ms=$(rand_ms 100 60000)
  steps=$(rand_steps)

  name="${NAMES[$(( (i - 1) % ${#NAMES[@]} ))]}"
  suffix=$(printf "%02d" "$i")
  full_name="${name}-${suffix}"

  body="{\"name\":\"${full_name}\",\"durationMs\":${ms},\"steps\":${steps}}"

  response=$(curl -sf -X POST "$ENDPOINT" \
    -H "Content-Type: application/json" \
    -d "$body")

  id=$(echo "$response" | extract_id)

  if [ "$steps" -eq 0 ]; then
    mode="indeterminate"
  else
    mode="${steps} steps"
  fi

  printf "  submitted %-28s  %6dms  %-16s  id=%.8s…\n" \
    "$full_name" "$ms" "$mode" "${id:-error}"
done

echo ""
echo "Jobs panel : $BASE/settings/jobs"
echo "API        : $BASE/api/v1/jobs"
