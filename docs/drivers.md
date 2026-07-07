# Drivers

`go-seeder` supports the following database drivers:

1. **PostgreSQL** (`postgres`)
2. **MySQL** (`mysql`)
3. **MongoDB** (`mongodb`)

## PostgreSQL
Uses `github.com/jackc/pgx/v5`. Connection URI format:
`postgres://user:password@host:port/dbname?query=parameters`

## MySQL
Uses `github.com/go-sql-driver/mysql`. Connection URI format:
`mysql://user:password@tcp(host:port)/dbname?query=parameters`

## MongoDB
Uses `go.mongodb.org/mongo-driver`. Connection URI format:
`mongodb://user:password@host:port/dbname?query=parameters`
