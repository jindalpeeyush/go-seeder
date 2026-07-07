package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jindalpeeyush/go-seeder"
)

func init() {
	// Register an Up/Down seed version 1720310400
	seeder.Register(&seeder.Seed{
		Version:   1720310400,
		Name:      "initialize_users",
		Direction: seeder.Up,
		Driver:    "postgres", // runs on postgres driver
		Run: func(ctx context.Context, db seeder.DB) error {
			fmt.Println("Applying initialize_users...")
			return db.InsertJSON(ctx, "users", []map[string]interface{}{
				{"name": "Admin", "role": "admin"},
				{"name": "User", "role": "member"},
			})
		},
	})

	seeder.Register(&seeder.Seed{
		Version:   1720310400,
		Name:      "initialize_users",
		Direction: seeder.Down,
		Driver:    "postgres",
		Run: func(ctx context.Context, db seeder.DB) error {
			fmt.Println("Rolling back initialize_users...")
			return db.DeleteJSON(ctx, "users", map[string]interface{}{
				"role": "admin",
			})
		},
	})
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/mydb?sslmode=disable"
	}

	fmt.Printf("Connecting to database with DSN: %s\n", dsn)
	
	// Initialize the programmatic seeder
	s, err := seeder.New(seeder.Options{
		DSN: dsn,
	})
	if err != nil {
		log.Fatalf("Failed to initialize seeder: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Apply pending seeds
	fmt.Println("Running up migrations...")
	if err := s.RunUp(ctx); err != nil {
		log.Fatalf("RunUp failed: %v", err)
	}
	fmt.Println("Seeding completed successfully.")
}
