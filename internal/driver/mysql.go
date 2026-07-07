package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLDriver struct {
	db *sql.DB
}

func (d *MySQLDriver) Name() string { return "mysql" }

func (d *MySQLDriver) Connect(ctx context.Context, dsn string) error {
	dsn = strings.TrimPrefix(dsn, "mysql://")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("mysql: failed to open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("mysql: failed to ping: %w", err)
	}
	d.db = db
	return nil
}

func (d *MySQLDriver) ExecSQL(ctx context.Context, query string) error {
	if d.db == nil {
		return fmt.Errorf("mysql: not connected")
	}
	_, err := d.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("mysql: exec failed: %w", err)
	}
	return nil
}

func (d *MySQLDriver) InsertJSON(ctx context.Context, table string, records []map[string]interface{}) error {
	if d.db == nil {
		return fmt.Errorf("mysql: not connected")
	}
	if len(records) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, rec := range records {
		cols, phs, vals := mysqlBuildInsert(rec)
		q := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", table, cols, phs)
		if _, err := tx.ExecContext(ctx, q, vals...); err != nil {
			return fmt.Errorf("mysql: insert into %s: %w", table, err)
		}
	}
	return tx.Commit()
}

func (d *MySQLDriver) DeleteJSON(ctx context.Context, table string, filter map[string]interface{}) error {
	if d.db == nil {
		return fmt.Errorf("mysql: not connected")
	}
	if len(filter) == 0 {
		return nil
	}
	conds, vals := mysqlBuildWhere(filter)
	q := fmt.Sprintf("DELETE FROM `%s` WHERE %s", table, conds)
	_, err := d.db.ExecContext(ctx, q, vals...)
	if err != nil {
		return fmt.Errorf("mysql: delete from %s: %w", table, err)
	}
	return nil
}

func (d *MySQLDriver) Truncate(ctx context.Context, tables ...string) error {
	if d.db == nil {
		return fmt.Errorf("mysql: not connected")
	}
	d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0")
	for _, t := range tables {
		if _, err := d.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE `%s`", t)); err != nil {
			d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")
			return fmt.Errorf("mysql: truncate %s: %w", t, err)
		}
	}
	d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")
	return nil
}

// --- Version tracking ---

func (d *MySQLDriver) CreateVersionTable(ctx context.Context) error {
	q := `CREATE TABLE IF NOT EXISTS seeder_versions (
		version    BIGINT PRIMARY KEY,
		seed_name  VARCHAR(255) NOT NULL,
		dirty      BOOLEAN NOT NULL DEFAULT FALSE,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := d.db.ExecContext(ctx, q)
	return err
}

func (d *MySQLDriver) GetAppliedVersions(ctx context.Context) ([]AppliedSeed, error) {
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

func (d *MySQLDriver) RecordVersion(ctx context.Context, version int64, name string, dirty bool) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO seeder_versions (version, seed_name, dirty, applied_at) VALUES (?, ?, ?, ?)",
		version, name, dirty, time.Now().UTC())
	return err
}

func (d *MySQLDriver) SetDirty(ctx context.Context, version int64, dirty bool) error {
	_, err := d.db.ExecContext(ctx,
		"UPDATE seeder_versions SET dirty = ? WHERE version = ?", dirty, version)
	return err
}

func (d *MySQLDriver) RemoveVersion(ctx context.Context, version int64) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM seeder_versions WHERE version = ?", version)
	return err
}

func (d *MySQLDriver) Close(_ context.Context) error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// --- helpers ---

func mysqlBuildInsert(rec map[string]interface{}) (cols, phs string, vals []interface{}) {
	c := make([]string, 0, len(rec))
	p := make([]string, 0, len(rec))
	vals = make([]interface{}, 0, len(rec))
	for k, v := range rec {
		c = append(c, fmt.Sprintf("`%s`", k))
		p = append(p, "?")
		vals = append(vals, v)
	}
	return strings.Join(c, ", "), strings.Join(p, ", "), vals
}

func mysqlBuildWhere(filter map[string]interface{}) (conds string, vals []interface{}) {
	cs := make([]string, 0, len(filter))
	vals = make([]interface{}, 0, len(filter))
	for k, v := range filter {
		cs = append(cs, fmt.Sprintf("`%s` = ?", k))
		vals = append(vals, v)
	}
	return strings.Join(cs, " AND "), vals
}
