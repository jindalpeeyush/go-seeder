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

	"github.com/jindalpeeyush/go-seeder/internal/driver"
)

// Direction type for seed direction.
type Direction string

const (
	Up   Direction = "up"
	Down Direction = "down"
)

// DB is the public interface for seed functions.
type DB interface {
	ExecSQL(ctx context.Context, query string) error
	InsertJSON(ctx context.Context, table string, records []map[string]interface{}) error
	DeleteJSON(ctx context.Context, table string, filter map[string]interface{}) error
	Truncate(ctx context.Context, tables ...string) error
}

// SeedFunc is a function that performs a seed operation.
type SeedFunc func(ctx context.Context, db DB) error

// Seed represents a registered Go seed.
type Seed struct {
	Version   int64
	Name      string
	Direction Direction
	Driver    string // postgres, mysql, mongodb
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

// Options for creating a Seeder.
type Options struct {
	Driver string
	DSN    string
}

// Seeder provides direct programmatic access.
type Seeder struct {
	driver driver.Driver
}

// New creates a Seeder and connects to the database.
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

// DB returns the database handle.
func (s *Seeder) DB() DB { return &dbAdapter{drv: s.driver} }

// RunUp runs all pending registered Up seeds matching this driver.
func (s *Seeder) RunUp(ctx context.Context) error {
	applied, err := s.driver.GetAppliedVersions(ctx)
	if err != nil {
		return err
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
		if err := s.driver.RecordVersion(ctx, seed.Version, seed.Name, true); err != nil {
			return err
		}
		if err := seed.Run(ctx, db); err != nil {
			return fmt.Errorf("seed %d_%s failed: %w", seed.Version, seed.Name, err)
		}
		// Mark clean
		if err := s.driver.SetDirty(ctx, seed.Version, false); err != nil {
			return err
		}
	}
	return nil
}

// RunDown rolls back the last N applied seeds.
func (s *Seeder) RunDown(ctx context.Context, steps int) error {
	applied, err := s.driver.GetAppliedVersions(ctx)
	if err != nil {
		return err
	}
	sort.Slice(applied, func(i, j int) bool {
		return applied[i].Version > applied[j].Version
	})
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
		if err := s.driver.SetDirty(ctx, a.Version, true); err != nil {
			return err
		}
		if err := seed.Run(ctx, db); err != nil {
			return fmt.Errorf("rollback %d_%s failed: %w", seed.Version, seed.Name, err)
		}
		if err := s.driver.RemoveVersion(ctx, a.Version); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the connection.
func (s *Seeder) Close() error {
	return s.driver.Close(context.Background())
}
