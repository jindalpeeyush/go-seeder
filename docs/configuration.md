# Configuration

`go-seeder` can be configured both programmatically via `seeder.Config` and via CLI flags.

## Programmatic Configuration
```go
s, err := seeder.New(seeder.Options{
	DSN: "postgres://user:pass@localhost:5432/db?sslmode=disable",
})
```

## CLI Configuration
- `-path`: Path to seed files directory
- `-database`: Database connection URI
- `-verbose`: Enable verbose output
- `--dry-run`: Preview operations without executing
