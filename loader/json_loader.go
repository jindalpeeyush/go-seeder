package loader

import (
	"encoding/json"
	"fmt"
	"os"
)

// loadJSON parses a JSON seed file. Format depends on direction:
//
// Up file (insert records):
//
//	{
//	  "driver": "mongodb",
//	  "table": "users",
//	  "records": [
//	    {"name": "Alice", "email": "alice@example.com"}
//	  ]
//	}
//
// Down file (truncate or delete):
//
//	{
//	  "driver": "mongodb",
//	  "table": "users",
//	  "truncate": true
//	}
//
//	{
//	  "driver": "mongodb",
//	  "table": "users",
//	  "delete": {"name": "Alice"}
//	}
func loadJSON(seed *SeedFile) error {
	data, err := os.ReadFile(seed.Path)
	if err != nil {
		return fmt.Errorf("failed to read JSON file %s: %w", seed.Path, err)
	}

	var raw jsonFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse JSON file %s: %w", seed.Path, err)
	}

	if raw.Table == "" {
		return fmt.Errorf("JSON seed %s is missing required 'table' field", seed.Path)
	}

	seed.Driver = raw.Driver
	seed.Table = raw.Table
	seed.Records = raw.Records
	seed.Truncate = raw.Truncate
	seed.Delete = raw.Delete

	return nil
}

// jsonFile is the JSON seed file structure.
type jsonFile struct {
	Driver   string                   `json:"driver"`
	Table    string                   `json:"table"`
	Records  []map[string]interface{} `json:"records"`  // up: records to insert
	Truncate bool                     `json:"truncate"` // down: truncate table
	Delete   map[string]interface{}   `json:"delete"`   // down: delete filter
}
