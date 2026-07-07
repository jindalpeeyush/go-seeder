# go-seeder

A pluggable database seeder for Go — seed your **PostgreSQL**, **MySQL**, or **MongoDB** databases using **SQL files**, **JSON files**, or **Go code** with versioned up/down migrations.

[![Go Reference](https://pkg.go.dev/badge/github.com/jindalpeeyush/go-seeder.svg)](https://pkg.go.dev/github.com/jindalpeeyush/go-seeder)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Features

- 🗄️ **Multi-database** — PostgreSQL, MySQL, MongoDB
- 📁 **Multi-format** — SQL files, JSON files, Go code
- ⬆️ **Up / Down** — Apply and rollback seeds with version tracking
- 🔢 **Version tracking** — Tracks applied seeds in `seeder_versions` table/collection
- 🖥️ **CLI tool** — Install globally, run from any project
- 📦 **Go library** — Import and use programmatically
- 🔍 **Dry-run mode** — Preview without executing

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

```bash
# Create a SQL seed for PostgreSQL
seeder create -driver=postgres -ext=sql -dir=database/seeders create_users

# Create a JSON seed for MongoDB
seeder create -driver=mongodb -ext=json -dir=database/seeders dummy_users

# Create a Go seed for MySQL (default ext is go)
seeder create -driver=mysql -dir=database/seeders add_admin
```

This generates timestamped files:
```
database/seeders/
├── 1720310400_create_users.sql
├── 1720310401_dummy_users.json
└── 1720310402_add_admin.go
```

### 2. Edit Seed Files

**SQL file** (`1720310400_create_users.sql`):
```sql
-- +seeder Up
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
INSERT INTO users (name, email) VALUES ('Bob', 'bob@example.com');

-- +seeder Down
DELETE FROM users WHERE email IN ('alice@example.com', 'bob@example.com');
```

**JSON file** (`1720310401_dummy_users.json`):
```json
{
  "table": "users",
  "up": [
    {"name": "Alice", "email": "alice@example.com"},
    {"name": "Bob", "email": "bob@example.com"}
  ],
  "down": {
    "truncate": true
  }
}
```

**Go file** (`1720310402_add_admin.go`):
```go
package seeds

import (
    "context"
    "github.com/jindalpeeyush/go-seeder/pkg/seeder"
)

func init() {
    seeder.Register(&seeder.Seed{
        Version: 1720310402,
        Name:    "add_admin",
        Up: func(ctx context.Context, db seeder.DB) error {
            return db.InsertJSON(ctx, "users", []map[string]interface{}{
                {"name": "Admin", "role": "admin"},
            })
        },
        Down: func(ctx context.Context, db seeder.DB) error {
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

# Force set version
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
| `create` | Create a new seed file |
| `up` | Apply all pending seeds |
| `down [N]` | Roll back seeds (last N, or all) |
| `force <ver>` | Force set database version |

### Create Flags

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `-driver` | Database driver: `postgres`, `mysql`, `mongodb` | **Yes** | — |
| `-ext` | File extension: `go`, `sql`, `json` | No | `go` |
| `-dir` | Output directory | No | `database/seeders` |

### Global Flags

| Flag | Description |
|------|-------------|
| `-path` | Path to seed files directory (required for up/down/force) |
| `-database` | Database connection URI (required for up/down/force) |
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

Use `-- +seeder Up` and `-- +seeder Down` markers:

```sql
-- +seeder Up
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100)
);
INSERT INTO products (name) VALUES ('Widget');

-- +seeder Down
DROP TABLE IF EXISTS products;
```

### JSON Files

**Single table:**
```json
{
  "table": "users",
  "up": [
    {"name": "Alice", "email": "alice@example.com"}
  ],
  "down": {
    "truncate": true
  }
}
```

**Multiple tables:**
```json
[
  {
    "table": "users",
    "up": [{"name": "Alice"}],
    "down": {"truncate": true}
  },
  {
    "table": "posts",
    "up": [{"title": "Hello", "author": "Alice"}],
    "down": {"delete": {"author": "Alice"}}
  }
]
```

**Down options:**
- `{"truncate": true}` — Truncates the table
- `{"delete": {"key": "value"}}` — Deletes matching records

### Go Files

Go seed files use `init()` to register seeds with the global registry. To execute them, build a custom binary that imports your seed package:

```go
// cmd/seed/main.go
package main

import (
    "context"
    "log"
    "os"

    "github.com/jindalpeeyush/go-seeder/pkg/seeder"
    _ "yourproject/database/seeders" // import seeds to trigger init()
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

// Register seeds
seeder.Register(&seeder.Seed{
    Version: 1720310400,
    Name:    "users",
    Up: func(ctx context.Context, db seeder.DB) error {
        return db.InsertJSON(ctx, "users", []map[string]interface{}{
            {"name": "Alice", "email": "alice@example.com"},
        })
    },
    Down: func(ctx context.Context, db seeder.DB) error {
        return db.Truncate(ctx, "users")
    },
})

// Apply pending seeds
s.RunUp(context.Background())

// Rollback last 2 seeds
s.RunDown(context.Background(), 2)
```

---

## Version Tracking

go-seeder automatically creates a `seeder_versions` table (or collection in MongoDB) to track which seeds have been applied:

| Column | Type | Description |
|--------|------|-------------|
| `version` | BIGINT | Seed timestamp (primary key) |
| `seed_name` | TEXT | Seed name |
| `applied_at` | TIMESTAMP | When the seed was applied |

---

## Notes

- **MongoDB + SQL**: SQL files are not supported with MongoDB. Use JSON or Go files instead.
- **Seed ordering**: Seeds are applied in ascending version (timestamp) order.
- **Transactions**: SQL databases use transactions for JSON inserts — if one record fails, the entire batch rolls back.
- **File naming**: `<unix_timestamp>_<seed_name>.<ext>` — the `create` command generates timestamps automatically.

---

## Contributing

1. Fork the repo
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

MIT License — see [LICENSE](LICENSE) for details.
