// seed populates the database with sample data for development.
//
// # Safety
//
// Seeding wipes ALL existing rows in every application table before
// inserting sample data. It is guarded accordingly:
//   - refuses to run when APP_ENV=production, and
//   - requires the explicit -force flag otherwise.
//
// The seeded accounts use well-known passwords (each account's email as its
// password); another reason this must never run against real data.
//
// # Architecture
//
// The seed delegates to internal/seed which uses the store layer for most
// entity creation and the service layer for business-logic-heavy operations
// (templates, custom fields). Only clearData and a few niche operations
// (task_custom_field_values, task_dependencies, channel_user_overrides)
// still use raw SQL because no store/service equivalent exists.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"ipmanlk/plume/internal/seed"
	"ipmanlk/plume/internal/store"
	"ipmanlk/plume/internal/store/migration"

	_ "modernc.org/sqlite"
)

func main() {
	force := flag.Bool("force", false, "required to proceed with seeding (it deletes all existing data)")
	flag.Parse()

	if os.Getenv("APP_ENV") == "production" {
		log.Fatal("refusing to seed: APP_ENV=production. The seeder destroys all existing data.")
	}
	if !*force {
		fmt.Fprintln(os.Stderr, "seeding WIPES all existing data in the database.")
		fmt.Fprintln(os.Stderr, "Re-run with -force to proceed.")
		os.Exit(1)
	}

	log.Println("🌱 Plume database seeder")

	// Load database path from env or use default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/plume.db"
	}

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Open database connection (reuses store.NewDB for consistent pragmas)
	db, err := store.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations so the schema exists before seeding
	if err := migration.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations applied.")

	// Delegate to the Seeder which uses store + service layer
	s := seed.NewSeeder(db)
	s.Seed()
}
