// Package cli implements the flag-based CLI for go-seeder.
// Uses Go's standard flag package for the golang-migrate-style syntax:
//
//	seeder -path=database/seeders -database "DB_URI" -verbose up
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jindalpeeyush/go-seeder/internal/seeder"
)

const usage = `Usage:
  seeder create -driver=<driver> [-ext=<ext>] [-dir=<dir>] <seed_name>
  seeder -path=<path> -database <uri> [-verbose] [--dry-run] up
  seeder -path=<path> -database <uri> [-verbose] [--dry-run] down [N]
  seeder -path=<path> -database <uri> [-verbose] [--dry-run] force <version>
  seeder -help

Commands:
  create    Create a new seed file
  up        Apply all pending seeds
  down      Roll back seeds (optionally N steps)
  force     Force set the database version

Create Flags:
  -driver   Database driver: postgres, mysql, mongodb (required)
  -ext      File extension: go, sql, json (default: go)
  -dir      Output directory (default: database/seeders)

Global Flags:
  -path     Path to seed files directory
  -database Database connection URI
  -verbose  Enable verbose output
  --dry-run Preview operations without executing

File Format:
  Seed files are named: <timestamp>_<seed_name>.<ext>
  Example: 1720310400_create_users.sql

Documentation: https://github.com/jindalpeeyush/go-seeder
`

// Run parses command-line arguments and dispatches to the appropriate handler.
func Run() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Check for -help / --help anywhere
	for _, arg := range os.Args[1:] {
		if arg == "-help" || arg == "--help" || arg == "-h" {
			fmt.Print(usage)
			os.Exit(0)
		}
	}

	// Check if first non-flag arg is "create" — it has its own flag set
	firstArg := findCommand(os.Args[1:])

	if firstArg == "create" {
		runCreate(os.Args[1:])
		return
	}

	// Parse global flags for up/down/force
	globalFlags := flag.NewFlagSet("seeder", flag.ExitOnError)
	globalFlags.Usage = func() { fmt.Print(usage) }

	path := globalFlags.String("path", "", "path to seed files directory")
	database := globalFlags.String("database", "", "database connection URI")
	verbose := globalFlags.Bool("verbose", false, "enable verbose output")
	dryRun := globalFlags.Bool("dry-run", false, "preview operations without executing")

	globalFlags.Parse(os.Args[1:])

	// Remaining args after flags
	args := globalFlags.Args()
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}

	command := args[0]
	cmdArgs := args[1:]

	// Validate required flags
	if *path == "" {
		exitError("missing required flag: -path")
	}
	if *database == "" {
		exitError("missing required flag: -database")
	}

	cfg := seeder.Config{
		Path:     *path,
		Database: *database,
		Verbose:  *verbose,
		DryRun:   *dryRun,
	}

	engine := seeder.NewEngine(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error
	switch command {
	case "up":
		err = engine.Up(ctx)
	case "down":
		steps := 0
		if len(cmdArgs) > 0 {
			steps, err = strconv.Atoi(cmdArgs[0])
			if err != nil {
				exitError("invalid step count %q: must be a number", cmdArgs[0])
			}
		}
		err = engine.Down(ctx, steps)
	case "force":
		if len(cmdArgs) == 0 {
			exitError("force requires a version argument: seeder ... force <version>")
		}
		version, parseErr := strconv.ParseInt(cmdArgs[0], 10, 64)
		if parseErr != nil {
			exitError("invalid version %q: must be a number", cmdArgs[0])
		}
		err = engine.Force(ctx, version)
	default:
		exitError("unknown command %q. Run 'seeder -help' for usage.", command)
	}

	if err != nil {
		exitError("%v", err)
	}
}

// findCommand finds the first non-flag argument in args.
func findCommand(args []string) string {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if len(arg) > 0 && arg[0] != '-' {
			return arg
		}
	}
	return ""
}

func exitError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
