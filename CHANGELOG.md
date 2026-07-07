# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-07

### Added
- **Multi-Database Support**: Drivers for PostgreSQL (`잭jackc/pgx`), MySQL (`go-sql-driver/mysql`), and MongoDB (`go.mongodb.org/mongo-driver`).
- **Multiple Formats**: Support for database seeding using SQL files, JSON files, and native Go code.
- **Version Tracking**: Track applied seeders automatically using the `seeder_versions` schema/collection.
- **Dirty Flag & Error Storing**: Automatically records detailed error logs (`why_dirty` column) on seed failure to keep state clean.
- **Self-Healing Rollbacks**: Permit calling rollback (`down`) on the latest dirty migration, automatically clearing the dirty flag once rollback succeeds.
- **Robust Halt on Failure**: If a rollback step itself fails, it remains dirty with the new error message, halting further rollbacks in the chain.
- **Programmatic & CLI APIs**: Fully-featured commands (`up`, `down [N]`, `force <version>`, `create`) accessible both through the command line and directly imported in Go projects.
