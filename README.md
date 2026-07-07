# go-seeder

A pluggable database seeder for Go — seed your **PostgreSQL**, **MySQL**, or **MongoDB** databases using **SQL files**, **JSON files**, or **Go code** with versioned up/down migrations.

[![Go Reference](https://pkg.go.dev/badge/github.com/jindalpeeyush/go-seeder.svg)](https://pkg.go.dev/github.com/jindalpeeyush/go-seeder)
[![Go Report Card](https://goreportcard.com/badge/github.com/jindalpeeyush/go-seeder)](https://goreportcard.com/report/github.com/jindalpeeyush/go-seeder)
[![CI Status](https://github.com/jindalpeeyush/go-seeder/actions/workflows/go.yml/badge.svg)](https://github.com/jindalpeeyush/go-seeder/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Features

- 🗄️ **Multi-database** — PostgreSQL, MySQL, MongoDB
- 📁 **Multi-format** — SQL files, JSON files, Go code
- ⬆️ **Up / Down** — Apply and rollback seeds with version tracking
- 🔢 **Version tracking** — Tracks applied seeds in `seeder_versions` table/collection with execution dirty state check
- 🖥️ **CLI tool** — Install globally, run from any project
- 📦 **Go library** — Import and use programmatically
- 🔍 **Dry-run mode** — Preview operations without executing

---

## Installation

### CLI Tool

```bash
go install github.com/jindalpeeyush/go-seeder/cmd/seeder@latest
```

### Go Library

```bash
go get github.com/jindalpeeyush/go-seeder
```

---

## Quick Start

### 1. Create Seed Files

Use `seeder create` to generate paired Up and Down seed files:

```bash
# Create a SQL seed for PostgreSQL
seeder create -driver=postgres -ext=sql -dir=database/seeders create_users

# Create a JSON seed for MongoDB
seeder create -driver=mongodb -ext=json -dir=database/seeders dummy_users

# Create Go seeds for MySQL (default ext is go)
seeder create -driver=mysql -dir=database/seeders add_admin
```

This generates timestamped file pairs:
```
database/seeders/
├── 1720310400_create_users.up.sql
├── 1720310400_create_users.down.sql
├── 1720310401_dummy_users.up.json
├── 1720310401_dummy_users.down.json
├── 1720310402_add_admin.up.go
└── 1720310402_add_admin.down.go
```

### 2. Edit Seed Files

#### SQL Files
SQL files include a driver header on the first line.

**Up file** (`1720310400_create_users.up.sql`):
```sql
-- driver: postgres
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
```

**Down file** (`1720310400_create_users.down.sql`):
```sql
-- driver: postgres
DELETE FROM users WHERE email = 'alice@example.com';
```

#### JSON Files
JSON files require a `"driver"` and a `"table"` property.

**Up file** (`1720310401_dummy_users.up.json`):
```json
{
  "driver": "mongodb",
  "table": "users",
  "records": [
    {"name": "Alice", "email": "alice@example.com"}
  ]
}
```

**Down file** (`1720310401_dummy_users.down.json`):
```json
{
  "driver": "mongodb",
  "table": "users",
  "truncate": true
}
```

#### Go Files
Go files register with the global seeder registry on import.

**Up file** (`1720310402_add_admin.up.go`):
```go
package seeds

import (
	"context"
	"github.com/jindalpeeyush/go-seeder/pkg/seeder"
)

func init() {
	seeder.Register(&seeder.Seed{
		Version:   1720310402,
		Name:      "add_admin",
		Direction: seeder.Up,
		Driver:    "mysql",
		Run: func(ctx context.Context, db seeder.DB) error {
			return db.InsertJSON(ctx, "users", []map[string]interface{}{
				{"name": "Admin", "role": "admin"},
			})
		},
	})
}
```

**Down file** (`1720310402_add_admin.down.go`):
```go
package seeds

import (
	"context"
	"github.com/jindalpeeyush/go-seeder/pkg/seeder"
)

func init() {
	seeder.Register(&seeder.Seed{
		Version:   1720310402,
		Name:      "add_admin",
		Direction: seeder.Down,
		Driver:    "mysql",
		Run: func(ctx context.Context, db seeder.DB) error {
			return db.ExecSQL(ctx, "DELETE FROM users WHERE role = 'admin'")
		},
	})
}
```

### 3. Run Seeds

```bash
# Apply all pending seeds
seeder -path=database/seeders -database "postgres://user:pass@localhost:5432/mydb?sslmode=disable" up

# Rollback last seed
seeder -path=database/seeders -database "postgres://user:pass@localhost:5432/mydb?sslmode=disable" down 1

# Rollback all seeds
seeder -path=database/seeders -database "postgres://user:pass@localhost:5432/mydb?sslmode=disable" down

# Force set version (marks target version as clean, clears later versions)
seeder -path=database/seeders -database "postgres://user:pass@localhost:5432/mydb?sslmode=disable" force 1720310400
```

---

## CLI Reference

```
seeder create -driver=<driver> [-ext=<ext>] [-dir=<dir>] <seed_name>
seeder -path=<path> -database <uri> [-verbose] [--dry-run] up
seeder -path=<path> -database <uri> [-verbose] [--dry-run] down [N]
seeder -path=<path> -database <uri> [-verbose] [--dry-run] force <version>
seeder -help
```

### Commands

| Command | Description |
|---------|-------------|
| `create` | Create a new pair of up and down seed files |
| `up` | Apply all pending seeds |
| `down [N]` | Roll back applied seeds (last N, or all) |
| `force <ver>` | Force set database version and clear dirty state |

### Create Flags

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `-driver` | Database driver: `postgres`, `mysql`, `mongodb` | **Yes** | — |
| `-ext` | File extension: `go`, `sql`, `json` | No | `go` |
| `-dir` | Output directory | No | `database/seeders` |

### Global Flags

| Flag | Description |
|------|-------------|
| `-path` | Path to seed files directory |
| `-database` | Database connection URI |
| `-verbose` | Enable verbose output |
| `--dry-run` | Preview operations without executing |

---

## Connection Strings

| Database | DSN Format |
|----------|------------|
| PostgreSQL | `postgres://user:password@localhost:5432/dbname?sslmode=disable` |
| MySQL | `user:password@tcp(localhost:3306)/dbname?parseTime=true` |
| MongoDB | `mongodb://user:password@localhost:27017/dbname` |

> **Note:** The driver is auto-detected from the DSN for `up`/`down`/`force` commands. The `-driver` flag is only required for `create`.

---

## Seed File Formats

### SQL Files
Supports PostgreSQL and MySQL. Must contain a `-- driver:` comment line.

**Up/Down file content example:**
```sql
-- driver: postgres
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100)
);
```

### JSON Files
Supports MongoDB.

**Up file content properties:**
- `driver`: must be `"mongodb"`
- `table`: collection name
- `records`: slice of document maps to insert

**Down file content properties:**
- `driver`: must be `"mongodb"`
- `table`: collection name
- `truncate`: set to `true` to drop/truncate the collection
- `delete`: filter document mapping to remove specific records

### Go Files
Go seed files use `init()` to register seeds with the global registry. To execute them in your application, build a custom binary that imports your seed package:

```go
// cmd/seed/main.go
package main

import (
    "context"
    "log"
    "os"

    "github.com/jindalpeeyush/go-seeder/pkg/seeder"
    _ "yourproject/database/seeders" // imports seeds to trigger init()
)

func main() {
    s, err := seeder.New(seeder.Options{
        DSN: os.Getenv("DATABASE_URL"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    if err := s.RunUp(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

---

## Go Library API

Use `go-seeder` programmatically in your Go application:

```go
import "github.com/jindalpeeyush/go-seeder/pkg/seeder"

// Create a seeder
s, err := seeder.New(seeder.Options{
    Driver: "postgres",
    DSN:    "postgres://localhost:5432/mydb?sslmode=disable",
})
defer s.Close()

// Register seeds programmatically
seeder.Register(&seeder.Seed{
    Version:   1720310400,
    Name:      "users",
    Direction: seeder.Up,
    Driver:    "postgres",
    Run: func(ctx context.Context, db seeder.DB) error {
        return db.InsertJSON(ctx, "users", []map[string]interface{}{
            {"name": "Alice", "email": "alice@example.com"},
        })
    },
})

// Apply pending seeds
s.RunUp(context.Background())

// Rollback last 2 seeds
s.RunDown(context.Background(), 2)
```

---

## Version Tracking

`go-seeder` automatically creates a `seeder_versions` table (or collection in MongoDB) to track applied version states:

| Column | Type | Description |
|--------|------|-------------|
| `version` | BIGINT | Seed timestamp (primary key) |
| `seed_name` | TEXT | Seed name |
| `dirty` | BOOLEAN | Flag indicating if version execution failed |
| `why_dirty` | TEXT / string | Error message if execution failed (`dirty` is true) |
| `applied_at` | TIMESTAMP | When the seed was applied |

### Dirty State & Recovery Behavior
If a seed execution fails, `go-seeder` marks the version as `dirty = true` and records the error description in `why_dirty`.
- **Applying Seeds (`up`)**: If any seed version is currently dirty, running `up` will fail immediately and display the reason (`why_dirty`) so that you know what failed.
- **Rolling Back (`down`)**: Unlike other migration engines that block entirely when a dirty state occurs, `go-seeder` allows you to run `down` to roll back the latest applied seed even if it is dirty. If that rollback succeeds, the dirty version is deleted and the database state becomes clean again.
- **Rollback Failures**: If a rollback step itself fails, the version remains dirty (with the new rollback error message stored in `why_dirty`) and execution stops immediately without processing subsequent rollback steps.

---

## Technical Details & Design Constraints
- **Driver limits**: MongoDB only supports `.json` and `.go` extensions. SQL databases support `.sql` and `.go` extensions.
- **Transactions**: SQL databases execute batches inside database transactions (auto-rollbacks on failures).
- **Ordering**: Seeds are executed in ascending timestamp version order.
- **File naming**: `<unix_timestamp>_<seed_name>.<direction>.<ext>` — generated automatically on `create`.

---

## License

MIT License — see [LICENSE](LICENSE) for details.
