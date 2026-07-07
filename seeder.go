// Package seeder provides the public Go API for programmatic database seeding.
//
// Users create Go seed files that register seeds via init() functions.
// Each seed has a direction (Up or Down) and a Run function.
//
// # Example
//
//	func init() {
//	    seeder.Register(&seeder.Seed{
//	        Version:   1720310400,
//	        Name:      "add_admin",
//	        Direction: seeder.Up,
//	        Driver:    "postgres",
//	        Run: func(ctx context.Context, db seeder.DB) error {
//	            return db.InsertJSON(ctx, "users", []map[string]interface{}{
//	                {"name": "Admin", "role": "admin"},
//	            })
//	        },
//	    })
//	}
package seeder

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/jindalpeeyush/go-seeder/driver"
)

// Direction represents the execution direction of a seed operation (Up or Down).
type Direction string

const (
	// Up indicates applying database seed data.
	Up   Direction = "up"
	// Down indicates rolling back/removing database seed data.
	Down Direction = "down"
)

// DB defines the database operations interface available inside a seed function.
type DB interface {
	// ExecSQL executes a raw SQL query statement (PostgreSQL and MySQL only).
	ExecSQL(ctx context.Context, query string) error
	// InsertJSON inserts a slice of records/documents into the specified table/collection.
	InsertJSON(ctx context.Context, table string, records []map[string]interface{}) error
	// DeleteJSON removes records/documents matching the filter map from the specified table/collection.
	DeleteJSON(ctx context.Context, table string, filter map[string]interface{}) error
	// Truncate deletes all records/documents from the specified tables/collections.
	Truncate(ctx context.Context, tables ...string) error
}

// SeedFunc defines the signature of a seed function execution hook.
type SeedFunc func(ctx context.Context, db DB) error

// Seed represents a single registered seed containing version metadata and execution logic.
type Seed struct {
	// Version is the unique version identifier of the seed (usually a Unix timestamp).
	Version   int64
	// Name is the descriptive name of the seed.
	Name      string
	// Direction specifies if the seed is an Up or Down migration.
	Direction Direction
	// Driver specifies the target database engine: "postgres", "mysql", or "mongodb".
	// If empty, the seed runs on any connected database.
	Driver    string // postgres, mysql, mongodb
	// Run is the function hook executed during the seed operation.
	Run       SeedFunc
}

// --- Global registry ---

var (
	mu       sync.Mutex
	registry []*Seed
)

// Register adds a seed to the global registry.
func Register(seed *Seed) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, seed)
}

// GetRegistered returns all registered seeds sorted by version.
func GetRegistered() []*Seed {
	mu.Lock()
	defer mu.Unlock()
	seeds := make([]*Seed, len(registry))
	copy(seeds, registry)
	sort.Slice(seeds, func(i, j int) bool {
		return seeds[i].Version < seeds[j].Version
	})
	return seeds
}

// ClearRegistry clears all registered seeds (for testing).
func ClearRegistry() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
}

// --- DB adapter ---

type dbAdapter struct{ drv driver.Driver }

func (a *dbAdapter) ExecSQL(ctx context.Context, q string) error {
	return a.drv.ExecSQL(ctx, q)
}
func (a *dbAdapter) InsertJSON(ctx context.Context, t string, r []map[string]interface{}) error {
	return a.drv.InsertJSON(ctx, t, r)
}
func (a *dbAdapter) DeleteJSON(ctx context.Context, t string, f map[string]interface{}) error {
	return a.drv.DeleteJSON(ctx, t, f)
}
func (a *dbAdapter) Truncate(ctx context.Context, t ...string) error {
	return a.drv.Truncate(ctx, t...)
}

// --- Programmatic API ---

// Options contains configuration options for initializing a new Seeder.
type Options struct {
	// Driver specifies the database driver. If empty, the driver is auto-detected from DSN.
	Driver string
	// DSN is the connection URI string.
	DSN    string
}

// Seeder provides direct programmatic control over database seed versions and execution.
type Seeder struct {
	driver driver.Driver
}

// New creates and initializes a Seeder, establishes a database connection,
// and ensures the version tracking schema is created.
func New(opts Options) (*Seeder, error) {
	var drv driver.Driver
	var err error
	if opts.Driver != "" {
		drv, err = driver.New(opts.Driver)
	} else {
		drv, err = driver.FromDSN(opts.DSN)
	}
	if err != nil {
		return nil, err
	}
	if err := drv.Connect(context.Background(), opts.DSN); err != nil {
		return nil, err
	}
	if err := drv.CreateVersionTable(context.Background()); err != nil {
		drv.Close(context.Background())
		return nil, err
	}
	return &Seeder{driver: drv}, nil
}

// DB returns a DB adapter wrapping the active database connection for manual seeding operations.
func (s *Seeder) DB() DB { return &dbAdapter{drv: s.driver} }

// RunUp executes all pending registered Up seeds matching this database driver in ascending version order.
// It fails immediately if any applied version in the database is currently marked dirty.
func (s *Seeder) RunUp(ctx context.Context) error {
	applied, err := s.driver.GetAppliedVersions(ctx)
	if err != nil {
		return err
	}
	for _, a := range applied {
		if a.Dirty {
			if a.WhyDirty != "" {
				return fmt.Errorf("version %d (%s) is dirty: %s — you need to down this version first to process",
					a.Version, a.Name, a.WhyDirty)
			}
			return fmt.Errorf("version %d (%s) is dirty — you need to down this version first to process",
				a.Version, a.Name)
		}
	}

	appliedMap := make(map[int64]bool)
	for _, a := range applied {
		appliedMap[a.Version] = true
	}

	db := &dbAdapter{drv: s.driver}
	driverName := s.driver.Name()

	for _, seed := range GetRegistered() {
		if seed.Direction != Up {
			continue
		}
		if seed.Driver != "" && seed.Driver != driverName {
			continue
		}
		if appliedMap[seed.Version] {
			continue
		}
		// Mark dirty
		if err := s.driver.RecordVersion(ctx, seed.Version, seed.Name, true, ""); err != nil {
			return err
		}
		if err := seed.Run(ctx, db); err != nil {
			if setErr := s.driver.SetDirty(ctx, seed.Version, true, err.Error()); setErr != nil {
				// ignore
			}
			return fmt.Errorf("seed %d_%s failed: %w", seed.Version, seed.Name, err)
		}
		// Mark clean
		if err := s.driver.SetDirty(ctx, seed.Version, false, ""); err != nil {
			return err
		}
	}
	return nil
}

// RunDown rolls back the last N applied seeds in descending version order.
// If steps is <= 0, it rolls back all applied seeds.
// It permits rolling back the latest version even if it is dirty,
// but returns an error if any older version in the list is dirty.
func (s *Seeder) RunDown(ctx context.Context, steps int) error {
	applied, err := s.driver.GetAppliedVersions(ctx)
	if err != nil {
		return err
	}
	sort.Slice(applied, func(i, j int) bool {
		return applied[i].Version > applied[j].Version
	})

	// Check for dirty.
	// Only the latest version (applied[0]) is allowed to be dirty.
	for i, a := range applied {
		if a.Dirty && i > 0 {
			if a.WhyDirty != "" {
				return fmt.Errorf("version %d (%s) is dirty: %s — you need to down newer versions first to process",
					a.Version, a.Name, a.WhyDirty)
			}
			return fmt.Errorf("version %d (%s) is dirty — you need to down newer versions first to process",
				a.Version, a.Name)
		}
	}

	if steps > 0 && steps < len(applied) {
		applied = applied[:steps]
	}

	// Build map of down seeds
	downMap := make(map[int64]*Seed)
	for _, seed := range GetRegistered() {
		if seed.Direction == Down {
			downMap[seed.Version] = seed
		}
	}

	db := &dbAdapter{drv: s.driver}

	for _, a := range applied {
		seed, ok := downMap[a.Version]
		if !ok {
			continue
		}
		if err := s.driver.SetDirty(ctx, a.Version, true, ""); err != nil {
			return err
		}
		if err := seed.Run(ctx, db); err != nil {
			if setErr := s.driver.SetDirty(ctx, a.Version, true, err.Error()); setErr != nil {
				// ignore
			}
			return fmt.Errorf("rollback %d_%s failed: %w", seed.Version, seed.Name, err)
		}
		if err := s.driver.RemoveVersion(ctx, a.Version); err != nil {
			return err
		}
	}
	return nil
}

// Close releases any database connections and resources held by the Seeder.
func (s *Seeder) Close() error {
	return s.driver.Close(context.Background())
}
