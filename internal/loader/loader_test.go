package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileName(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantVersion int64
		wantName    string
		wantDir     Direction
		wantExt     string
		wantErr     bool
	}{
		{"up sql", "1720310400_users.up.sql", 1720310400, "users", Up, "sql", false},
		{"down sql", "1720310400_users.down.sql", 1720310400, "users", Down, "sql", false},
		{"up json", "1720310400_data.up.json", 1720310400, "data", Up, "json", false},
		{"down json", "1720310400_data.down.json", 1720310400, "data", Down, "json", false},
		{"up go", "1720310400_admin.up.go", 1720310400, "admin", Up, "go", false},
		{"compound name", "1720310400_create_users.up.sql", 1720310400, "create_users", Up, "sql", false},
		{"with dir", "seeds/1720310400_test.up.sql", 1720310400, "test", Up, "sql", false},
		{"no direction", "1720310400_test.sql", 0, "", "", "", true},
		{"bad direction", "1720310400_test.foo.sql", 0, "", "", "", true},
		{"no underscore", "1720310400.up.sql", 0, "", "", "", true},
		{"bad timestamp", "abc_test.up.sql", 0, "", "", "", true},
		{"empty name", "1720310400_.up.sql", 0, "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, name, dir, ext, err := ParseFileName(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if ver != tt.wantVersion {
				t.Errorf("version = %d, want %d", ver, tt.wantVersion)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if dir != tt.wantDir {
				t.Errorf("direction = %q, want %q", dir, tt.wantDir)
			}
			if ext != tt.wantExt {
				t.Errorf("ext = %q, want %q", ext, tt.wantExt)
			}
		})
	}
}

func TestIsSupportedFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"1720310400_test.up.sql", true},
		{"1720310400_test.down.sql", true},
		{"1720310400_test.up.json", true},
		{"1720310400_test.down.json", true},
		{"1720310400_test.up.go", false},   // Go not executed by CLI
		{"1720310400_test.sql", false},      // missing direction
		{"test.csv", false},
	}
	for _, tt := range tests {
		if got := IsSupportedFile(tt.path); got != tt.want {
			t.Errorf("IsSupportedFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestLoadSQL_Up(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1720310400_users.up.sql")

	content := `-- driver: postgres

INSERT INTO users (name) VALUES ('Alice');
INSERT INTO users (name) VALUES ('Bob');
`
	os.WriteFile(path, []byte(content), 0o644)

	seed, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if seed.Version != 1720310400 {
		t.Errorf("version = %d", seed.Version)
	}
	if seed.Direction != Up {
		t.Errorf("direction = %q", seed.Direction)
	}
	if seed.Driver != "postgres" {
		t.Errorf("driver = %q", seed.Driver)
	}
	if len(seed.Statements) != 2 {
		t.Errorf("statements = %d, want 2", len(seed.Statements))
	}
}

func TestLoadSQL_Down(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1720310400_users.down.sql")

	content := `-- driver: mysql

DELETE FROM users WHERE name IN ('Alice', 'Bob');
`
	os.WriteFile(path, []byte(content), 0o644)

	seed, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if seed.Direction != Down {
		t.Errorf("direction = %q", seed.Direction)
	}
	if seed.Driver != "mysql" {
		t.Errorf("driver = %q", seed.Driver)
	}
	if len(seed.Statements) != 1 {
		t.Errorf("statements = %d, want 1", len(seed.Statements))
	}
}

func TestLoadJSON_Up(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1720310400_users.up.json")

	content := `{
		"driver": "mongodb",
		"table": "users",
		"records": [
			{"name": "Alice", "email": "alice@example.com"}
		]
	}`
	os.WriteFile(path, []byte(content), 0o644)

	seed, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if seed.Driver != "mongodb" {
		t.Errorf("driver = %q", seed.Driver)
	}
	if seed.Table != "users" {
		t.Errorf("table = %q", seed.Table)
	}
	if len(seed.Records) != 1 {
		t.Errorf("records = %d, want 1", len(seed.Records))
	}
}

func TestLoadJSON_Down_Truncate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1720310400_users.down.json")

	content := `{
		"driver": "mongodb",
		"table": "users",
		"truncate": true
	}`
	os.WriteFile(path, []byte(content), 0o644)

	seed, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !seed.Truncate {
		t.Error("expected truncate = true")
	}
}

func TestLoadJSON_MissingTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1720310400_bad.up.json")

	content := `{"records": [{"name": "Alice"}]}`
	os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for missing table")
	}
}
