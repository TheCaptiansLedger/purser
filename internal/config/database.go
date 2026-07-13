package config

import "fmt"

// Database selects and configures Purser's persistence backend — see
// docs/adr/0012-datastore-persistence.md for the Datastore abstraction
// this backs, and docs/adr/0010-configuration.md for the Viper
// conventions this struct follows.
type Database struct {
	// Driver selects the backend: "badger", "postgres", "mysql", or
	// "sqlite".
	Driver string       `mapstructure:"driver"`
	Badger BadgerConfig `mapstructure:"badger"`
	SQL    SQLConfig    `mapstructure:"sql"`
}

// BadgerConfig configures the BadgerDB backend
// (internal/adapters/datastore/badger).
type BadgerConfig struct {
	// DataDir is the directory Badger stores its LSM-tree files in. Left
	// empty here means "derive from Paths.DataDir" — see Config.Load.
	DataDir string `mapstructure:"data_dir"`
	// ValueLogDir optionally places the value log on a separate disk.
	// Empty means "same as DataDir".
	ValueLogDir string `mapstructure:"value_log_dir"`
	// SyncWrites flushes every write to disk before returning. Off by
	// default for throughput; on trades throughput for durability.
	SyncWrites bool `mapstructure:"sync_writes"`
}

// SQLConfig configures the SQL backend
// (internal/adapters/datastore/sql). Postgres, MySQL, and SQLite share
// this one DSN-shaped struct; Database.Driver picks which dialect the DSN
// is opened against.
type SQLConfig struct {
	// DSN is the connection string/path for the selected dialect. Left
	// empty here for the sqlite driver means "derive from
	// Paths.DataDir" — see Config.Load. Postgres/MySQL have no sane
	// default and must be set explicitly.
	DSN string `mapstructure:"dsn"`
}

// DefaultDatabase returns Database's defaults: the badger driver, with
// Badger.DataDir/SQL.DSN left empty so Config.DefaultConfig can derive
// them from Paths.DataDir.
func DefaultDatabase() Database {
	return Database{Driver: "badger"}
}

// Validate checks Database's invariants.
func (d Database) Validate() error {
	switch d.Driver {
	case "badger", "postgres", "mysql", "sqlite":
	default:
		return fmt.Errorf("database.driver must be one of badger, postgres, mysql, sqlite, got %q", d.Driver)
	}
	if d.Driver == "postgres" || d.Driver == "mysql" {
		if d.SQL.DSN == "" {
			return fmt.Errorf("database.sql.dsn must not be empty for driver %q", d.Driver)
		}
	}
	return nil
}
