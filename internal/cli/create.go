package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// --- Templates ---

const sqlUpTemplate = `-- driver: {{.Driver}}

-- TODO: Add your seed data here
`

const sqlDownTemplate = `-- driver: {{.Driver}}

-- TODO: Add your rollback logic here
`

const jsonUpTemplate = `{
  "driver": "{{.Driver}}",
  "table": "{{.Name}}",
  "records": []
}
`

const jsonDownTemplate = `{
  "driver": "{{.Driver}}",
  "table": "{{.Name}}",
  "truncate": true
}
`

const goUpTemplate = `package seeds

import (
	"context"

	"github.com/jindalpeeyush/go-seeder/pkg/seeder"
)

func init() {
	seeder.Register(&seeder.Seed{
		Version:   {{.Version}},
		Name:      "{{.Name}}",
		Direction: seeder.Up,
		Driver:    "{{.Driver}}",
		Run: func(ctx context.Context, db seeder.DB) error {
			// TODO: implement seed logic
			// Example:
			// return db.InsertJSON(ctx, "{{.Name}}", []map[string]interface{}{
			//     {"key": "value"},
			// })
			return nil
		},
	})
}
`

const goDownTemplate = `package seeds

import (
	"context"

	"github.com/jindalpeeyush/go-seeder/pkg/seeder"
)

func init() {
	seeder.Register(&seeder.Seed{
		Version:   {{.Version}},
		Name:      "{{.Name}}",
		Direction: seeder.Down,
		Driver:    "{{.Driver}}",
		Run: func(ctx context.Context, db seeder.DB) error {
			// TODO: implement rollback logic
			// Example:
			// return db.Truncate(ctx, "{{.Name}}")
			return nil
		},
	})
}
`

type templateData struct {
	Version int64
	Name    string
	Driver  string
}

// runCreate handles "seeder create".
func runCreate(args []string) {
	createIdx := -1
	for i, arg := range args {
		if arg == "create" {
			createIdx = i
			break
		}
	}
	if createIdx < 0 {
		exitError("internal error: create not found")
	}

	fs := flag.NewFlagSet("create", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: seeder create -driver=<driver> [-ext=<ext>] [-dir=<dir>] <seed_name>")
		fmt.Println("\nFlags:")
		fs.PrintDefaults()
	}

	driverFlag := fs.String("driver", "", "database driver: postgres, mysql, mongodb (required)")
	extFlag := fs.String("ext", "go", "file extension: go, sql, json")
	dirFlag := fs.String("dir", "database/seeders", "output directory")
	fs.Parse(args[createIdx+1:])

	// Validate driver (required)
	if *driverFlag == "" {
		exitError("-driver is required (postgres, mysql, mongodb)")
	}
	validDrivers := map[string]bool{"postgres": true, "mysql": true, "mongodb": true}
	if !validDrivers[*driverFlag] {
		exitError("invalid driver %q: must be postgres, mysql, or mongodb", *driverFlag)
	}

	// Validate ext
	validExts := map[string]bool{"go": true, "sql": true, "json": true}
	if !validExts[*extFlag] {
		exitError("invalid extension %q: must be go, sql, or json", *extFlag)
	}

	// Driver/extension compatibility
	if *driverFlag == "mongodb" && *extFlag == "sql" {
		exitError("MongoDB does not support SQL; use -ext=json or -ext=go")
	}
	if (*driverFlag == "postgres" || *driverFlag == "mysql") && *extFlag == "json" {
		exitError("%s does not support JSON seeds; use -ext=sql or -ext=go", *driverFlag)
	}

	// Seed name
	remaining := fs.Args()
	if len(remaining) == 0 {
		exitError("missing seed name. Usage: seeder create -driver=<driver> <seed_name>")
	}
	seedName := remaining[0]

	// Generate both up and down files
	timestamp := time.Now().Unix()
	data := templateData{Version: timestamp, Name: seedName, Driver: *driverFlag}

	if err := os.MkdirAll(*dirFlag, 0o755); err != nil {
		exitError("failed to create directory: %v", err)
	}

	// Select templates based on extension
	var upTmpl, downTmpl string
	switch *extFlag {
	case "sql":
		upTmpl, downTmpl = sqlUpTemplate, sqlDownTemplate
	case "json":
		upTmpl, downTmpl = jsonUpTemplate, jsonDownTemplate
	case "go":
		upTmpl, downTmpl = goUpTemplate, goDownTemplate
	}

	// Write up file
	upFile := filepath.Join(*dirFlag, fmt.Sprintf("%d_%s.up.%s", timestamp, seedName, *extFlag))
	writeTemplate(upFile, upTmpl, data)

	// Write down file
	downFile := filepath.Join(*dirFlag, fmt.Sprintf("%d_%s.down.%s", timestamp, seedName, *extFlag))
	writeTemplate(downFile, downTmpl, data)

	fmt.Printf("✓ Created seed files:\n")
	fmt.Printf("  ↑ %s\n", upFile)
	fmt.Printf("  ↓ %s\n", downFile)
	fmt.Printf("  Driver: %s | Version: %d\n", *driverFlag, timestamp)
}

func writeTemplate(path, tmplContent string, data templateData) {
	if _, err := os.Stat(path); err == nil {
		exitError("file already exists: %s", path)
	}

	tmpl, err := template.New("seed").Parse(tmplContent)
	if err != nil {
		exitError("template error: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		exitError("failed to create %s: %v", path, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		exitError("failed to write %s: %v", path, err)
	}
}
