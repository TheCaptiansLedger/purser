# Configuration

YAML file (default: `purser.yaml`, override with `$CONFIG_PATH`) + environment variable overrides (12-factor).

## Schema

```yaml
server:
  port: 7474

database:
  driver: sqlite        # sqlite | postgres
  dsn: purser.db        # file path for sqlite, connection string for postgres

library:
  root: /media          # base path for organized media

log:
  level: info           # debug | info | warn | error
  format: text          # text | json
```

## Environment Variable Overrides

All config keys map to `PURSER_<SECTION>_<KEY>` env vars (uppercase, underscores). For example:

- `PURSER_SERVER_PORT`
- `PURSER_DATABASE_DRIVER`
- `PURSER_DATABASE_DSN`
- `PURSER_LOG_LEVEL`

Source-specific API keys follow the same pattern:

- `PURSER_STASHDB_API_KEY`
- `PURSER_TMDB_API_KEY`
- `PURSER_TVDB_API_KEY`
