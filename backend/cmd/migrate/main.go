package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"chakuchuri/backend/internal/platform/config"
	"chakuchuri/backend/internal/platform/migrations"
)

func main() {
	cfg := config.Load()
	databaseURL := flag.String("database", cfg.DatabaseURL, "PostgreSQL connection string. Defaults to DATABASE_URL.")
	dir := flag.String("dir", cfg.MigrationsDir, "Migration directory.")
	steps := flag.Int("steps", 1, "Number of migrations to rollback with down.")
	flag.Parse()

	action := "status"
	if flag.NArg() > 0 {
		action = flag.Arg(0)
	}

	if *databaseURL == "" {
		log.Fatal("DATABASE_URL is required. Example: postgres://user:pass@localhost:5432/chakuchuri?sslmode=disable")
	}

	db, err := sql.Open("pgx", *databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	var statuses []migrations.Status
	switch action {
	case "up":
		statuses, err = migrations.Apply(ctx, db, *dir)
	case "down":
		statuses, err = migrations.Rollback(ctx, db, *dir, *steps)
	case "status":
		statuses, err = migrations.CurrentStatus(ctx, db, *dir)
	default:
		log.Fatalf("unknown action %q. Use up, down or status.", action)
	}
	if err != nil {
		log.Fatal(err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"ok":         true,
		"action":     action,
		"migrations": statuses,
	}); err != nil {
		fmt.Println("{\"ok\":true}")
	}
}
