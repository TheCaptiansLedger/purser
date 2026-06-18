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
- **Acquisition pipeline** - search configured indexers for missing content and dispatch to tooling such as disk search, torrent, or nzb and track.
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

## Legal

Purser is a metadata and library management tool. It does not host, distribute, stream, or cache any copyrighted content. Search and acquisition features connect to third-party tools (Prowlarr, download clients) that users configure and operate independently. Users are solely responsible for ensuring their use of Purser complies with applicable laws in their jurisdiction.

---

## License

[GNU Affero General Public License v3.0](LICENSE)

The AGPL ensures that anyone who deploys Purser as a hosted service must also release their modifications as open source. See [LICENSE](LICENSE) for the full text.
