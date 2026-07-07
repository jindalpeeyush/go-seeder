package seeder

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jindalpeeyush/go-seeder/internal/driver"
)

func TestEngine_Up(t *testing.T) {
	driver.GlobalMockDriver.Reset()
	dir := t.TempDir()

	// Create seed files
	// 1. up sql
	up1 := filepath.Join(dir, "1720310400_users.up.sql")
	os.WriteFile(up1, []byte("-- driver: mock\nINSERT INTO users (name) VALUES ('Alice');"), 0o644)

	// 2. up json
	up2 := filepath.Join(dir, "1720310401_posts.up.json")
	os.WriteFile(up2, []byte(`{
		"driver": "mock",
		"table": "posts",
		"records": [{"title": "Hello"}]
	}`), 0o644)

	cfg := Config{
		Path:     dir,
		Database: "mock://localhost",
		Verbose:  true,
	}

	engine := NewEngine(cfg)
	var logBuf bytes.Buffer
	engine.SetOutput(&logBuf)

	ctx := context.Background()
	if err := engine.Up(ctx); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	// Verify driver state
	drv := engine.driver.(*driver.MockDriver)
	if !drv.Connected {
		t.Error("driver not connected")
	}

	// Verify statements & records applied
	if len(drv.SQLs) != 1 || drv.SQLs[0] != "INSERT INTO users (name) VALUES ('Alice')" {
		t.Errorf("unexpected SQLs applied: %v", drv.SQLs)
	}

	if len(drv.Inserts["posts"]) != 1 || drv.Inserts["posts"][0]["title"] != "Hello" {
		t.Errorf("unexpected inserts applied: %v", drv.Inserts)
	}

	// Verify versions recorded as clean
	if len(drv.Versions) != 2 {
		t.Fatalf("expected 2 versions applied, got %d", len(drv.Versions))
	}
	if drv.Versions[0].Version != 1720310400 || drv.Versions[0].Dirty {
		t.Errorf("unexpected version 0: %+v", drv.Versions[0])
	}
	if drv.Versions[1].Version != 1720310401 || drv.Versions[1].Dirty {
		t.Errorf("unexpected version 1: %+v", drv.Versions[1])
	}
}

func TestEngine_Up_DirtySafety(t *testing.T) {
	driver.GlobalMockDriver.Reset()
	dir := t.TempDir()

	// Create dirty version in mock driver
	cfg := Config{
		Path:     dir,
		Database: "mock://localhost",
	}

	engine := NewEngine(cfg)
	var logBuf bytes.Buffer
	engine.SetOutput(&logBuf)

	ctx := context.Background()
	if err := engine.connect(ctx); err != nil {
		t.Fatal(err)
	}

	drv := engine.driver.(*driver.MockDriver)
	drv.RecordVersion(ctx, 1720310400, "failed_seed", true) // DIRTY

	// Create a new seed to run
	up1 := filepath.Join(dir, "1720310401_new.up.sql")
	os.WriteFile(up1, []byte("-- driver: mock\nSELECT 1;"), 0o644)

	// Trying to run Up should fail with dirty error
	err := engine.Up(ctx)
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("expected dirty error, got: %v", err)
	}
}

func TestEngine_Down(t *testing.T) {
	driver.GlobalMockDriver.Reset()
	dir := t.TempDir()

	// Create down files
	down1 := filepath.Join(dir, "1720310400_users.down.sql")
	os.WriteFile(down1, []byte("-- driver: mock\nDELETE FROM users;"), 0o644)

	down2 := filepath.Join(dir, "1720310401_posts.down.json")
	os.WriteFile(down2, []byte(`{
		"driver": "mock",
		"table": "posts",
		"truncate": true
	}`), 0o644)

	cfg := Config{
		Path:     dir,
		Database: "mock://localhost",
	}

	engine := NewEngine(cfg)
	var logBuf bytes.Buffer
	engine.SetOutput(&logBuf)

	ctx := context.Background()
	if err := engine.connect(ctx); err != nil {
		t.Fatal(err)
	}

	drv := engine.driver.(*driver.MockDriver)
	drv.RecordVersion(ctx, 1720310400, "users", false)
	drv.RecordVersion(ctx, 1720310401, "posts", false)

	// Rollback 1 step (should rollback 1720310401/posts)
	if err := engine.Down(ctx, 1); err != nil {
		t.Fatalf("Down(1) failed: %v", err)
	}

	if len(drv.Truncated) != 1 || drv.Truncated[0] != "posts" {
		t.Errorf("expected posts truncated, got: %v", drv.Truncated)
	}

	if len(drv.Versions) != 1 || drv.Versions[0].Version != 1720310400 {
		t.Errorf("expected only 1720310400 left, got: %v", drv.Versions)
	}
}

func TestEngine_Force(t *testing.T) {
	driver.GlobalMockDriver.Reset()
	dir := t.TempDir()

	cfg := Config{
		Path:     dir,
		Database: "mock://localhost",
	}

	engine := NewEngine(cfg)
	var logBuf bytes.Buffer
	engine.SetOutput(&logBuf)

	ctx := context.Background()
	if err := engine.connect(ctx); err != nil {
		t.Fatal(err)
	}

	drv := engine.driver.(*driver.MockDriver)
	drv.RecordVersion(ctx, 1720310400, "users", false)
	drv.RecordVersion(ctx, 1720310401, "posts", true) // dirty version

	// Force to 1720310400 (should remove 1720310401, clean dirty flag)
	if err := engine.Force(ctx, 1720310400); err != nil {
		t.Fatalf("Force failed: %v", err)
	}

	if len(drv.Versions) != 1 || drv.Versions[0].Version != 1720310400 || drv.Versions[0].Dirty {
		t.Errorf("expected version 1720310400 only and not dirty: %v", drv.Versions)
	}
}
