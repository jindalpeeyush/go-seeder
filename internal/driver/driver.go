// Package driver defines the database driver interface and provides
// factory functions to create drivers by name or auto-detect from DSN.
package driver

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AppliedSeed represents a seed recorded in seeder_versions.
type AppliedSeed struct {
	Version   int64
	Name      string
	Dirty     bool
	AppliedAt time.Time
}

// Driver is the interface that all database drivers must implement.
type Driver interface {
	Connect(ctx context.Context, dsn string) error
	ExecSQL(ctx context.Context, query string) error
	InsertJSON(ctx context.Context, table string, records []map[string]interface{}) error
	DeleteJSON(ctx context.Context, table string, filter map[string]interface{}) error
	Truncate(ctx context.Context, tables ...string) error

	// Version tracking
	CreateVersionTable(ctx context.Context) error
	GetAppliedVersions(ctx context.Context) ([]AppliedSeed, error)
	RecordVersion(ctx context.Context, version int64, name string, dirty bool) error
	SetDirty(ctx context.Context, version int64, dirty bool) error
	RemoveVersion(ctx context.Context, version int64) error

	Close(ctx context.Context) error
	Name() string
}

// New creates a Driver by name.
func New(name string) (Driver, error) {
	switch name {
	case "postgres":
		return &PostgresDriver{}, nil
	case "mysql":
		return &MySQLDriver{}, nil
	case "mongodb", "mongo":
		return &MongoDriver{}, nil
	case "mock":
		return GlobalMockDriver, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %q (supported: postgres, mysql, mongodb, mock)", name)
	}
}

// FromDSN auto-detects the driver from the DSN string.
func FromDSN(dsn string) (Driver, error) {
	lower := strings.ToLower(dsn)
	switch {
	case strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://"):
		return &PostgresDriver{}, nil
	case strings.HasPrefix(lower, "mongodb://") || strings.HasPrefix(lower, "mongodb+srv://"):
		return &MongoDriver{}, nil
	case strings.Contains(lower, "@tcp(") || strings.HasPrefix(lower, "mysql://"):
		return &MySQLDriver{}, nil
	case strings.HasPrefix(lower, "mock://"):
		return GlobalMockDriver, nil
	default:
		return nil, fmt.Errorf("cannot auto-detect driver from DSN; supported URI schemes: postgres://, mongodb://, mysql://, mock://")
	}
}
