#!/usr/bin/env bash
# Runs once, only when the postgres data directory is empty (the official
# postgres image's /docker-entrypoint-initdb.d/ convention). Creates two
# isolated roles+databases in the one Postgres instance: one for the app,
# one for Grafana's own backend — see
# docs/adr/0018-local-development-environment.md. Neither role can see the
# other's database.
set -euo pipefail

: "${PURSER_DB_USER:?PURSER_DB_USER must be set}"
: "${PURSER_DB_PASSWORD:?PURSER_DB_PASSWORD must be set}"
: "${PURSER_DB_NAME:?PURSER_DB_NAME must be set}"
: "${GRAFANA_DB_USER:?GRAFANA_DB_USER must be set}"
: "${GRAFANA_DB_PASSWORD:?GRAFANA_DB_PASSWORD must be set}"
: "${GRAFANA_DB_NAME:?GRAFANA_DB_NAME must be set}"

create_role_and_db() {
  local role="$1" password="$2" db="$3"

  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-SQL
    CREATE ROLE "${role}" WITH LOGIN PASSWORD '${password}';
    CREATE DATABASE "${db}" OWNER "${role}";
    REVOKE ALL ON DATABASE "${db}" FROM PUBLIC;
SQL
}

create_role_and_db "$PURSER_DB_USER" "$PURSER_DB_PASSWORD" "$PURSER_DB_NAME"
create_role_and_db "$GRAFANA_DB_USER" "$GRAFANA_DB_PASSWORD" "$GRAFANA_DB_NAME"
