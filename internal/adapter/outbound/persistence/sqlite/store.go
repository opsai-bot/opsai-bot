package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/jonny/opsai-bot/internal/adapter/outbound/persistence/sqlite/migration"
)

// validJournalModes defines accepted SQLite journal modes.
var validJournalModes = map[string]bool{
	"wal": true, "delete": true, "truncate": true,
	"persist": true, "memory": true, "off": true,
}

// Config holds SQLite connection configuration.
type Config struct {
	Path               string
	MaxOpenConns       int
	PragmaJournalMode  string
	PragmaBusyTimeout  int
}

// Store wraps a *sql.DB and exposes it for repository use.
type Store struct {
	DB *sql.DB
}

// NewStore opens the SQLite database at cfg.Path, applies pragmas, and runs migrations.
func NewStore(cfg Config) (*Store, error) {
	if cfg.PragmaJournalMode != "" && !validJournalModes[strings.ToLower(cfg.PragmaJournalMode)] {
		return nil, fmt.Errorf("invalid pragma journal mode: %q", cfg.PragmaJournalMode)
	}
	dsn := fmt.Sprintf(
		"%s?_journal_mode=%s&_busy_timeout=%d",
		cfg.Path,
		cfg.PragmaJournalMode,
		cfg.PragmaBusyTimeout,
	)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	if err := migration.Run(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &Store{DB: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.DB.Close() }
