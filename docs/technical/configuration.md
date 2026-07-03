# Configuration

YAML file (default: `purser.yaml`, override with `$CONFIG_PATH`) + environment variable overrides (12-factor).

## Schema

```yaml
server:
  port: 7474

storage:
  driver: badger        # badger | sqlite | postgres
  # badger: set data_dir (and optionally value_log_dir)
  data_dir: /data
  value_log_dir: ""     # defaults to data_dir when empty
  # sqlite: set dsn to a file path
  # postgres: set dsn to a connection string
  dsn: ""

library:
  root: /media          # base path for organized media

log:
  level: info           # debug | info | warn | error
  format: text          # text | json
```

## Environment Variable Overrides

All config keys map to `PURSER_<SECTION>_<KEY>` env vars (uppercase, underscores). For example:

- `PURSER_SERVER_PORT`
- `PURSER_STORAGE_DRIVER`
- `PURSER_STORAGE_DATA_DIR`
- `PURSER_STORAGE_DSN`
- `PURSER_LOG_LEVEL`

Source-specific API keys follow the same pattern:

- `PURSER_STASHDB_API_KEY`
- `PURSER_TMDB_API_KEY`
- `PURSER_TVDB_API_KEY`
