package driver

import (
	"context"
	"time"
)

// MockDriver is a driver implementation for testing and debugging.
type MockDriver struct {
	Connected bool
	DSN       string
	SQLs      []string
	Inserts   map[string][]map[string]interface{}
	Deletes   map[string][]map[string]interface{}
	Truncated []string
	Versions  []AppliedSeed
	DirtyMap  map[int64]bool
}

// GlobalMockDriver is a shared instance of MockDriver used for testing.
var GlobalMockDriver = NewMockDriver()

func NewMockDriver() *MockDriver {
	return &MockDriver{
		Inserts:  make(map[string][]map[string]interface{}),
		Deletes:  make(map[string][]map[string]interface{}),
		DirtyMap: make(map[int64]bool),
	}
}

// Reset clears all recorded states on the mock driver.
func (m *MockDriver) Reset() {
	m.Connected = false
	m.DSN = ""
	m.SQLs = nil
	m.Inserts = make(map[string][]map[string]interface{})
	m.Deletes = make(map[string][]map[string]interface{})
	m.Truncated = nil
	m.Versions = nil
	m.DirtyMap = make(map[int64]bool)
}

func (m *MockDriver) Name() string { return "mock" }

func (m *MockDriver) Connect(ctx context.Context, dsn string) error {
	m.Connected = true
	m.DSN = dsn
	return nil
}

func (m *MockDriver) ExecSQL(ctx context.Context, query string) error {
	m.SQLs = append(m.SQLs, query)
	return nil
}

func (m *MockDriver) InsertJSON(ctx context.Context, table string, records []map[string]interface{}) error {
	m.Inserts[table] = append(m.Inserts[table], records...)
	return nil
}

func (m *MockDriver) DeleteJSON(ctx context.Context, table string, filter map[string]interface{}) error {
	m.Deletes[table] = append(m.Deletes[table], filter)
	return nil
}

func (m *MockDriver) Truncate(ctx context.Context, tables ...string) error {
	m.Truncated = append(m.Truncated, tables...)
	return nil
}

func (m *MockDriver) CreateVersionTable(ctx context.Context) error {
	return nil
}

func (m *MockDriver) GetAppliedVersions(ctx context.Context) ([]AppliedSeed, error) {
	return m.Versions, nil
}

func (m *MockDriver) RecordVersion(ctx context.Context, version int64, name string, dirty bool) error {
	m.Versions = append(m.Versions, AppliedSeed{
		Version:   version,
		Name:      name,
		Dirty:     dirty,
		AppliedAt: time.Now().UTC(),
	})
	m.DirtyMap[version] = dirty
	return nil
}

func (m *MockDriver) SetDirty(ctx context.Context, version int64, dirty bool) error {
	for i, v := range m.Versions {
		if v.Version == version {
			m.Versions[i].Dirty = dirty
		}
	}
	m.DirtyMap[version] = dirty
	return nil
}

func (m *MockDriver) RemoveVersion(ctx context.Context, version int64) error {
	var remaining []AppliedSeed
	for _, v := range m.Versions {
		if v.Version != version {
			remaining = append(remaining, v)
		}
	}
	m.Versions = remaining
	delete(m.DirtyMap, version)
	return nil
}

func (m *MockDriver) Close(ctx context.Context) error {
	return nil
}
