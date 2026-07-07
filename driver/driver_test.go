package driver

import (
	"testing"
)

func TestFromDSN(t *testing.T) {
	tests := []struct {
		dsn      string
		wantName string
		wantErr  bool
	}{
		{"postgres://user:pass@localhost:5432/db", "postgres", false},
		{"postgresql://user:pass@localhost:5432/db", "postgres", false},
		{"mongodb://user:pass@localhost:27017/db", "mongodb", false},
		{"mongodb+srv://user:pass@cluster.example.com/db", "mongodb", false},
		{"user:pass@tcp(localhost:3306)/db", "mysql", false},
		{"mysql://user:pass@localhost:3306/db", "mysql", false},
		{"unknown://something", "", true},
		{"just-a-string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			drv, err := FromDSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromDSN(%q) error = %v, wantErr = %v", tt.dsn, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if drv.Name() != tt.wantName {
				t.Errorf("driver.Name() = %q, want %q", drv.Name(), tt.wantName)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"postgres", false},
		{"mysql", false},
		{"mongodb", false},
		{"mongo", false},
		{"sqlite", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drv, err := New(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("New(%q) error = %v, wantErr = %v", tt.name, err, tt.wantErr)
				return
			}
			if err == nil && drv == nil {
				t.Error("New() returned nil driver without error")
			}
		})
	}
}
