// Package seeder provides the core engine for versioned seed operations.
package seeder

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/jindalpeeyush/go-seeder/driver"
	"github.com/jindalpeeyush/go-seeder/loader"
)

// Config holds runtime configuration.
type Config struct {
	Path     string
	Database string
	Verbose  bool
	DryRun   bool
}

// Engine is the core seeder orchestrator.
type Engine struct {
	cfg    Config
	driver driver.Driver
	log    *log.Logger
}

func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg, log: log.New(os.Stdout, "", 0)}
}

// SetOutput sets logger output (for testing).
func (e *Engine) SetOutput(w io.Writer) { e.log.SetOutput(w) }

// Up applies all pending seeds.
func (e *Engine) Up(ctx context.Context) error {
	if err := e.connect(ctx); err != nil {
		return err
	}
	if !e.cfg.DryRun {
		defer e.driver.Close(ctx)
	}

	// Check for dirty state
	if !e.cfg.DryRun {
		if err := e.checkDirty(ctx); err != nil {
			return err
		}
	}

	// Discover up files
	upFiles, err := e.discoverDirection(loader.Up)
	if err != nil {
		return err
	}

	// Filter already applied
	pending, err := e.filterPending(ctx, upFiles)
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		e.log.Println("No pending seeds")
		return nil
	}

	e.log.Printf("Found %d pending seed(s)", len(pending))

	for _, seed := range pending {
		if err := e.applyUp(ctx, seed); err != nil {
			return fmt.Errorf("seed %d_%s failed: %w", seed.Version, seed.Name, err)
		}
	}

	e.log.Printf("✓ Applied %d seed(s)", len(pending))
	return nil
}

// Down rolls back seeds. steps=0 means all.
func (e *Engine) Down(ctx context.Context, steps int) error {
	if err := e.connect(ctx); err != nil {
		return err
	}
	if !e.cfg.DryRun {
		defer e.driver.Close(ctx)
	}

	applied, err := e.getAppliedReversed(ctx)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		e.log.Println("No seeds to roll back")
		return nil
	}

	if !e.cfg.DryRun {
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
	}

	if steps > 0 && steps < len(applied) {
		applied = applied[:steps]
	}

	e.log.Printf("Rolling back %d seed(s)", len(applied))

	for _, a := range applied {
		if err := e.applyDown(ctx, a); err != nil {
			return fmt.Errorf("rollback %d_%s failed: %w", a.Version, a.Name, err)
		}
	}

	e.log.Printf("✓ Rolled back %d seed(s)", len(applied))
	return nil
}

// Force sets the version without running seeds.
func (e *Engine) Force(ctx context.Context, version int64) error {
	if err := e.connect(ctx); err != nil {
		return err
	}
	if !e.cfg.DryRun {
		defer e.driver.Close(ctx)
	}

	if e.cfg.DryRun {
		e.log.Printf("[dry-run] Would force version to %d", version)
		return nil
	}

	// Remove all versions above forced version
	applied, _ := e.driver.GetAppliedVersions(ctx)
	for _, a := range applied {
		if a.Version > version {
			e.driver.RemoveVersion(ctx, a.Version)
		}
		// Clear dirty on the forced version
		if a.Version == version && a.Dirty {
			e.driver.SetDirty(ctx, version, false, "")
		}
	}

	e.log.Printf("✓ Version forced to %d", version)
	return nil
}

// --- internals ---

func (e *Engine) connect(ctx context.Context) error {
	if e.driver == nil {
		drv, err := driver.FromDSN(e.cfg.Database)
		if err != nil {
			return err
		}
		e.driver = drv
	}

	if e.cfg.DryRun {
		e.log.Printf("[dry-run] Would connect to %s", e.driver.Name())
		return nil
	}

	e.log.Printf("Connecting to %s...", e.driver.Name())
	if err := e.driver.Connect(ctx, e.cfg.Database); err != nil {
		return err
	}
	return e.driver.CreateVersionTable(ctx)
}

func (e *Engine) checkDirty(ctx context.Context) error {
	applied, err := e.driver.GetAppliedVersions(ctx)
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
	return nil
}

func (e *Engine) discoverDirection(dir loader.Direction) ([]*loader.SeedFile, error) {
	var files []*loader.SeedFile

	err := filepath.Walk(e.cfg.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !loader.IsSupportedFile(path) {
			return err
		}
		seed, err := loader.Load(path)
		if err != nil {
			return err
		}
		if seed.Direction == dir {
			// Only include files matching the connected driver
			if e.matchesDriver(seed) {
				files = append(files, seed)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", e.cfg.Path, err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})
	return files, nil
}

func (e *Engine) matchesDriver(seed *loader.SeedFile) bool {
	if seed.Driver == "" {
		return true // no driver specified, assume compatible
	}
	return seed.Driver == e.driver.Name()
}

func (e *Engine) filterPending(ctx context.Context, files []*loader.SeedFile) ([]*loader.SeedFile, error) {
	if e.cfg.DryRun {
		return files, nil
	}

	applied, err := e.driver.GetAppliedVersions(ctx)
	if err != nil {
		return nil, err
	}
	appliedMap := make(map[int64]bool)
	for _, a := range applied {
		appliedMap[a.Version] = true
	}

	var pending []*loader.SeedFile
	for _, f := range files {
		if !appliedMap[f.Version] {
			pending = append(pending, f)
		}
	}
	return pending, nil
}

func (e *Engine) getAppliedReversed(ctx context.Context) ([]driver.AppliedSeed, error) {
	if e.cfg.DryRun {
		return nil, nil
	}
	applied, err := e.driver.GetAppliedVersions(ctx)
	if err != nil {
		return nil, err
	}
	// Reverse: newest first
	sort.Slice(applied, func(i, j int) bool {
		return applied[i].Version > applied[j].Version
	})
	return applied, nil
}

func (e *Engine) applyUp(ctx context.Context, seed *loader.SeedFile) error {
	e.log.Printf("  ↑ %d_%s.up.%s", seed.Version, seed.Name, seed.Ext)

	if e.cfg.DryRun {
		e.dryRunLog(seed)
		return nil
	}

	// Mark as dirty before executing
	if err := e.driver.RecordVersion(ctx, seed.Version, seed.Name, true, ""); err != nil {
		return err
	}

	if err := e.executeSeed(ctx, seed); err != nil {
		if setErr := e.driver.SetDirty(ctx, seed.Version, true, err.Error()); setErr != nil {
			e.log.Printf("Failed to set dirty error message: %v", setErr)
		}
		return err // leaves dirty=true for recovery
	}

	// Mark clean
	return e.driver.SetDirty(ctx, seed.Version, false, "")
}

func (e *Engine) applyDown(ctx context.Context, applied driver.AppliedSeed) error {
	// Find the .down file
	downFile := e.findFile(applied.Version, applied.Name, loader.Down)

	if downFile == nil {
		e.log.Printf("  ↓ %d_%s (no down file, removing record only)", applied.Version, applied.Name)
		if !e.cfg.DryRun {
			return e.driver.RemoveVersion(ctx, applied.Version)
		}
		return nil
	}

	e.log.Printf("  ↓ %d_%s.down.%s", applied.Version, applied.Name, downFile.Ext)

	if e.cfg.DryRun {
		e.dryRunLog(downFile)
		return nil
	}

	// Mark dirty
	if err := e.driver.SetDirty(ctx, applied.Version, true, ""); err != nil {
		return err
	}

	if err := e.executeSeed(ctx, downFile); err != nil {
		if setErr := e.driver.SetDirty(ctx, applied.Version, true, err.Error()); setErr != nil {
			e.log.Printf("Failed to set dirty error message: %v", setErr)
		}
		return err
	}

	return e.driver.RemoveVersion(ctx, applied.Version)
}

func (e *Engine) executeSeed(ctx context.Context, seed *loader.SeedFile) error {
	switch seed.Ext {
	case "sql":
		for _, stmt := range seed.Statements {
			e.verbose("    SQL: %s", truncate(stmt, 80))
			if err := e.driver.ExecSQL(ctx, stmt); err != nil {
				return err
			}
		}
	case "json":
		if len(seed.Records) > 0 {
			e.verbose("    INSERT %d record(s) into %q", len(seed.Records), seed.Table)
			if err := e.driver.InsertJSON(ctx, seed.Table, seed.Records); err != nil {
				return err
			}
		}
		if seed.Truncate {
			e.verbose("    TRUNCATE %q", seed.Table)
			if err := e.driver.Truncate(ctx, seed.Table); err != nil {
				return err
			}
		}
		if len(seed.Delete) > 0 {
			e.verbose("    DELETE from %q", seed.Table)
			if err := e.driver.DeleteJSON(ctx, seed.Table, seed.Delete); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) findFile(version int64, name string, dir loader.Direction) *loader.SeedFile {
	pattern := filepath.Join(e.cfg.Path, fmt.Sprintf("%d_%s.%s.*", version, name, dir))
	matches, _ := filepath.Glob(pattern)
	for _, m := range matches {
		if loader.IsSupportedFile(m) {
			seed, err := loader.Load(m)
			if err == nil && e.matchesDriver(seed) {
				return seed
			}
		}
	}
	return nil
}

func (e *Engine) dryRunLog(seed *loader.SeedFile) {
	for _, stmt := range seed.Statements {
		e.log.Printf("    [dry-run] SQL: %s", truncate(stmt, 80))
	}
	if len(seed.Records) > 0 {
		e.log.Printf("    [dry-run] INSERT %d record(s) into %q", len(seed.Records), seed.Table)
	}
	if seed.Truncate {
		e.log.Printf("    [dry-run] TRUNCATE %q", seed.Table)
	}
	if len(seed.Delete) > 0 {
		e.log.Printf("    [dry-run] DELETE from %q", seed.Table)
	}
}

func (e *Engine) verbose(format string, args ...interface{}) {
	if e.cfg.Verbose {
		e.log.Printf(format, args...)
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
