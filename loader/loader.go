// Package loader provides seed file parsing for go-seeder.
//
// Seed files follow the naming convention:
//
//	<timestamp>_<seed_name>.up.<ext>
//	<timestamp>_<seed_name>.down.<ext>
//
// Each file contains a single direction (up OR down) with a driver header.
package loader

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Direction represents seed direction.
type Direction string

const (
	Up   Direction = "up"
	Down Direction = "down"
)

// SeedFile represents a parsed seed file.
type SeedFile struct {
	Version   int64
	Name      string
	Path      string
	Direction Direction
	Ext       string // sql, json, go
	Driver    string // postgres, mysql, mongodb (extracted from file content)

	// SQL statements (for .sql files)
	Statements []string

	// JSON data (for .json files)
	Table    string
	Records  []map[string]interface{} // for up: insert records
	Truncate bool                     // for down: truncate table
	Delete   map[string]interface{}   // for down: delete filter
}

// ParseFileName extracts version, name, direction, and extension from a filename.
// Format: <timestamp>_<seed_name>.up.<ext> or <timestamp>_<seed_name>.down.<ext>
func ParseFileName(filename string) (version int64, name string, dir Direction, ext string, err error) {
	base := filepath.Base(filename)

	// Split from right: ext, direction, rest
	// Example: 1720310400_create_users.up.sql
	//   parts after split by ".": ["1720310400_create_users", "up", "sql"]
	parts := strings.Split(base, ".")
	if len(parts) < 3 {
		return 0, "", "", "", fmt.Errorf("invalid seed filename %q: expected <timestamp>_<name>.up|down.<ext>", base)
	}

	ext = parts[len(parts)-1]
	dirStr := parts[len(parts)-2]
	nameWithVersion := strings.Join(parts[:len(parts)-2], ".")

	// Parse direction
	switch dirStr {
	case "up":
		dir = Up
	case "down":
		dir = Down
	default:
		return 0, "", "", "", fmt.Errorf("invalid seed filename %q: expected .up or .down before extension", base)
	}

	// Parse version and name from "1720310400_create_users"
	idx := strings.Index(nameWithVersion, "_")
	if idx < 0 {
		return 0, "", "", "", fmt.Errorf("invalid seed filename %q: expected <timestamp>_<name>", base)
	}

	versionStr := nameWithVersion[:idx]
	name = nameWithVersion[idx+1:]

	version, err = strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return 0, "", "", "", fmt.Errorf("invalid seed filename %q: timestamp %q is not a valid number", base, versionStr)
	}

	if name == "" {
		return 0, "", "", "", fmt.Errorf("invalid seed filename %q: seed name is empty", base)
	}

	return version, name, dir, ext, nil
}

// Load parses a seed file and returns a SeedFile.
func Load(path string) (*SeedFile, error) {
	version, name, dir, ext, err := ParseFileName(path)
	if err != nil {
		return nil, err
	}

	seed := &SeedFile{
		Version:   version,
		Name:      name,
		Path:      path,
		Direction: dir,
		Ext:       ext,
	}

	switch ext {
	case "sql":
		if err := loadSQL(seed); err != nil {
			return nil, err
		}
	case "json":
		if err := loadJSON(seed); err != nil {
			return nil, err
		}
	case "go":
		// Go files are executed via the library, not parsed by CLI
		return seed, nil
	default:
		return nil, fmt.Errorf("unsupported extension: .%s (supported: .sql, .json, .go)", ext)
	}

	return seed, nil
}

// IsSupportedFile checks if a file is a seed file the CLI can execute.
func IsSupportedFile(path string) bool {
	base := filepath.Base(path)
	parts := strings.Split(base, ".")
	if len(parts) < 3 {
		return false
	}

	ext := parts[len(parts)-1]
	dir := parts[len(parts)-2]

	if dir != "up" && dir != "down" {
		return false
	}

	return ext == "sql" || ext == "json"
}

// VersionKey returns the base key for pairing up/down files: "<version>_<name>"
func (s *SeedFile) VersionKey() string {
	return fmt.Sprintf("%d_%s", s.Version, s.Name)
}
