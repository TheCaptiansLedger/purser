# Purser

[![Domain Coverage](https://codecov.io/gh/TheCaptiansLedger/purser/graph/badge.svg?component=domain)](https://codecov.io/gh/TheCaptiansLedger/purser?component=domain)
[![Adapters Coverage](https://codecov.io/gh/TheCaptiansLedger/purser/graph/badge.svg?component=adapters)](https://codecov.io/gh/TheCaptiansLedger/purser?component=adapters)
[![API Coverage](https://codecov.io/gh/TheCaptiansLedger/purser/graph/badge.svg?component=api)](https://codecov.io/gh/TheCaptiansLedger/purser?component=api)

**Purser is a self-hosted metadata manager for the media you care about.**

It maintains a rich, browsable library enriched from external sources — TMDB, TVDB, MusicBrainz, StashDB, and others — and gives you tools to identify gaps in your collection and fill them. Think of the metadata side as a local mirror of everything worth knowing about your library; the acquisition side as a disciplined way to go get what you're missing.

Runs as a single binary, a container, or a Kubernetes workload. No cloud account required, no data leaves your server.

---

## What it does

- **Rich metadata library** — browse movies, TV, music, adult content, and JAV by title, performer, studio, tag, or any metadata dimension your sources provide
- **Multi-source enrichment** — pull from TMDB, TVDB, MusicBrainz, StashDB, TPDB, and more; external IDs kept for round-trip sync
- **Hierarchy-aware** — correct terminology and depth per content type: season/episode for TV, album/track for music, series/scene for adult, and so on
- **Collection tracking** — monitor at any level (studio, series, performer, individual item); control what's wanted with `all`, `future`, `latest`, or `none` modes
- **Acquisition pipeline** *(Phase 2)* — search indexers via Prowlarr, dispatch grabs to qBittorrent or Transmission, organize files on disk using user-defined naming templates
- **Modules** — enable only the content types you want; AfterDark and JAV are disabled by default

---

## Quick start

```bash
# Run with defaults (SQLite, port 7474)
./purser

# Open http://localhost:7474
```

Purser looks for `purser.yaml` in the working directory. Override with `$CONFIG_PATH`.

### Docker / Podman

```bash
docker run -p 7474:7474 -v ./data:/data ghcr.io/purser-app/purser
```

---

## Configuration

```yaml
server:
  port: 7474

database:
  driver: sqlite        # sqlite | postgres
  dsn: /data/purser.db

library:
  root: /media

log:
  level: info           # debug | info | warn | error
  format: text          # text | json

modules:
  movies: true
  tv: true
  music: true
  books: true
  afterdark: false      # adult content; disabled by default
  jav: false
```

All keys map to `PURSER_`-prefixed environment variables (`PURSER_SERVER_PORT`, `PURSER_MODULES_AFTERDARK_ENABLED`, etc.).

---

## Building from source

Requires Go 1.23+ and Node 18+.

```bash
# Backend only (serves pre-built UI if present)
go run ./cmd/purser

# Full build — UI + embedded static assets
cd web && npm install && npm run build && cd ..
go build -o purser ./cmd/purser

# Tests
go test ./...

# Regenerate sqlc types after SQL changes
sqlc generate
```

---

## Architecture

Hexagonal. Business logic in `internal/domain` and `internal/app` has no knowledge of HTTP, SQLite, or any external API. Everything outside connects through ports in `internal/ports`.

```
domain ← ports ← app ← adapters / api
```

| Layer | Path | Responsibility |
|---|---|---|
| Domain | `internal/domain` | Types and rules; no external imports |
| Ports | `internal/ports` | Go interfaces only |
| App services | `internal/app` | Use cases; depend only on ports |
| Adapters | `internal/adapters` | SQLite, metadata sources, download clients, filesystem |
| API | `internal/api` | HTTP handlers (Chi); translates HTTP ↔ app services |
| UI | `web/` | React 18 + TypeScript + Tailwind; `go:embed`ed into the binary |

---

## Verifying commits

All commits to this repository are signed. Public keys are in the [`keys/`](keys/) directory.

| Key | Fingerprint | Used for |
|-----|-------------|----------|
| `keys/theacaptiansledger.asc` | `F18A 78D6 A6FB 8E47 F01D  0426 9C04 0FFC 9921 D766` | Human commits |
| `keys/purser-ci.asc` | `DCC7 3B8D 68B8 F098 D878  F016 2F9A 9A18 67E9 D043` | Automated release commits |

```bash
# Import both keys
gpg --import keys/theacaptiansledger.asc
gpg --import keys/purser-ci.asc

# Verify a commit
git verify-commit HEAD

# Verify a tag
git verify-tag v1.0.0
```

---

## Legal

Purser is a metadata and library management tool. It does not host, distribute, stream, or cache any copyrighted content. Search and acquisition features connect to third-party tools (Prowlarr, download clients) that users configure and operate independently. Users are solely responsible for ensuring their use of Purser complies with applicable laws in their jurisdiction.

---

## License

[GNU Affero General Public License v3.0](LICENSE)

The AGPL ensures that anyone who deploys Purser as a hosted service must also release their modifications as open source. See [LICENSE](LICENSE) for the full text.
