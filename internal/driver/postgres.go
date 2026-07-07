package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresDriver struct {
	db *sql.DB
}

func (d *PostgresDriver) Name() string { return "postgres" }

func (d *PostgresDriver) Connect(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("postgres: failed to open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("postgres: failed to ping: %w", err)
	}
	d.db = db
	return nil
}

func (d *PostgresDriver) ExecSQL(ctx context.Context, query string) error {
	if d.db == nil {
		return fmt.Errorf("postgres: not connected")
	}
	_, err := d.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("postgres: exec failed: %w", err)
	}
	return nil
}

func (d *PostgresDriver) InsertJSON(ctx context.Context, table string, records []map[string]interface{}) error {
	if d.db == nil {
		return fmt.Errorf("postgres: not connected")
	}
	if len(records) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, rec := range records {
		cols, phs, vals := pgBuildInsert(rec)
		q := fmt.Sprintf("INSERT INTO %q (%s) VALUES (%s)", table, cols, phs)
		if _, err := tx.ExecContext(ctx, q, vals...); err != nil {
			return fmt.Errorf("postgres: insert into %s: %w", table, err)
		}
	}
	return tx.Commit()
}

func (d *PostgresDriver) DeleteJSON(ctx context.Context, table string, filter map[string]interface{}) error {
	if d.db == nil {
		return fmt.Errorf("postgres: not connected")
	}
	if len(filter) == 0 {
		return nil
	}
	conds, vals := pgBuildWhere(filter)
	q := fmt.Sprintf("DELETE FROM %q WHERE %s", table, conds)
	_, err := d.db.ExecContext(ctx, q, vals...)
	if err != nil {
		return fmt.Errorf("postgres: delete from %s: %w", table, err)
	}
	return nil
}

func (d *PostgresDriver) Truncate(ctx context.Context, tables ...string) error {
	if d.db == nil {
		return fmt.Errorf("postgres: not connected")
	}
	for _, t := range tables {
		if _, err := d.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %q CASCADE", t)); err != nil {
			return fmt.Errorf("postgres: truncate %s: %w", t, err)
		}
	}
	return nil
}

// --- Version tracking ---

func (d *PostgresDriver) CreateVersionTable(ctx context.Context) error {
	q := `CREATE TABLE IF NOT EXISTS seeder_versions (
		version    BIGINT PRIMARY KEY,
		seed_name  TEXT NOT NULL,
		dirty      BOOLEAN NOT NULL DEFAULT FALSE,
		applied_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`
	_, err := d.db.ExecContext(ctx, q)
	return err
}

func (d *PostgresDriver) GetAppliedVersions(ctx context.Context) ([]AppliedSeed, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT version, seed_name, dirty, applied_at FROM seeder_versions ORDER BY version ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seeds []AppliedSeed
	for rows.Next() {
		var s AppliedSeed
		if err := rows.Scan(&s.Version, &s.Name, &s.Dirty, &s.AppliedAt); err != nil {
			return nil, err
		}
		seeds = append(seeds, s)
	}
	return seeds, rows.Err()
}

func (d *PostgresDriver) RecordVersion(ctx context.Context, version int64, name string, dirty bool) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO seeder_versions (version, seed_name, dirty, applied_at) VALUES ($1, $2, $3, $4)",
		version, name, dirty, time.Now().UTC())
	return err
}

func (d *PostgresDriver) SetDirty(ctx context.Context, version int64, dirty bool) error {
	_, err := d.db.ExecContext(ctx,
		"UPDATE seeder_versions SET dirty = $1 WHERE version = $2", dirty, version)
	return err
}

func (d *PostgresDriver) RemoveVersion(ctx context.Context, version int64) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM seeder_versions WHERE version = $1", version)
	return err
}

func (d *PostgresDriver) Close(_ context.Context) error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// --- helpers ---

func pgBuildInsert(rec map[string]interface{}) (cols, phs string, vals []interface{}) {
	c := make([]string, 0, len(rec))
	p := make([]string, 0, len(rec))
	vals = make([]interface{}, 0, len(rec))
	i := 1
	for k, v := range rec {
		c = append(c, fmt.Sprintf("%q", k))
		p = append(p, fmt.Sprintf("$%d", i))
		vals = append(vals, v)
		i++
	}
	return strings.Join(c, ", "), strings.Join(p, ", "), vals
}

func pgBuildWhere(filter map[string]interface{}) (conds string, vals []interface{}) {
	cs := make([]string, 0, len(filter))
	vals = make([]interface{}, 0, len(filter))
	i := 1
	for k, v := range filter {
		cs = append(cs, fmt.Sprintf("%q = $%d", k, i))
		vals = append(vals, v)
		i++
	}
	return strings.Join(cs, " AND "), vals
}
