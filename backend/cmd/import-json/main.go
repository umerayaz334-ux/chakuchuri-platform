// import-json imports an offline snapshot into an empty, migrated candidate database.
// A failed candidate must never be promoted to the live API.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/backups"
	"chakuchuri/backend/internal/customers"
	"chakuchuri/backend/internal/directory"
	emailops "chakuchuri/backend/internal/email"
	"chakuchuri/backend/internal/files"
	"chakuchuri/backend/internal/ratesheets"
	"chakuchuri/backend/internal/workflow"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type source struct {
	Users struct {
		Version int               `json:"version"`
		Users   []auth.UserRecord `json:"users"`
	}
	Customers struct {
		Version   int                         `json:"version"`
		Customers []customers.Customer        `json:"customers"`
		Requests  []customers.DocumentRequest `json:"documentRequests"`
	}
	Files struct {
		Version int                `json:"version"`
		Files   []files.FileRecord `json:"files"`
	}
	Workflow  workflow.State
	Email     emailops.State
	Directory directory.State
	Rates     ratesheets.State
}

type result struct {
	Section         string   `json:"section"`
	SourceCount     int      `json:"sourceCount"`
	TargetCount     int      `json:"targetCount"`
	Match           bool     `json:"match"`
	DifferentFields []string `json:"differentFields,omitempty"`
}

func main() {
	data := flag.String("source-dir", "", "Offline JSON snapshot directory (required)")
	storage := flag.String("storage-dir", "", "Original upload storage for the safety backup")
	backup := flag.String("backup-dir", "", "Dedicated safety backup directory, outside source/storage")
	apply := flag.Bool("apply", false, "Import into an empty database; otherwise only reconcile")
	prepare := flag.String("prepare-dir", "", "Write a separately reviewed migration copy; never edit the source")
	flag.Parse()
	if *prepare != "" {
		if *apply {
			fmt.Fprintln(os.Stderr, "prepare and apply are separate operations")
			os.Exit(1)
		}
		if err := prepareSource(*data, *prepare); err != nil {
			fmt.Fprintln(os.Stderr, "preparation refused:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*data, *storage, *backup, *apply); err != nil {
		fmt.Fprintln(os.Stderr, "migration refused:", err)
		os.Exit(1)
	}
}

func run(data, storage, backup string, apply bool) error {
	if data == "" || os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("source-dir and DATABASE_URL are required")
	}
	s, hashes, err := loadSource(data)
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	if apply {
		if err := validateSource(s); err != nil {
			return err
		}
		if backup == "" || storage == "" {
			return fmt.Errorf("apply requires backup-dir and storage-dir")
		}
		var occupied bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users UNION ALL SELECT 1 FROM customers UNION ALL SELECT 1 FROM files UNION ALL SELECT 1 FROM workflow_state_meta)`).Scan(&occupied); err != nil {
			return err
		}
		if occupied {
			return fmt.Errorf("target is not empty; refusing to overwrite or merge an existing database")
		}
		manager := backups.NewManager(backups.Options{DataDir: data, StorageDir: storage, BackupDir: backup, Retention: 14})
		record, err := manager.Create("json-to-postgres-migration")
		if err != nil {
			return fmt.Errorf("safety backup: %w", err)
		}
		fmt.Println("Safety backup:", record.Name)
		steps := []struct {
			name string
			save func() error
		}{
			{"customers", func() error {
				return customers.NewPostgresRepository(db).SaveCustomers(s.Customers.Customers, s.Customers.Requests)
			}},
			{"users", func() error { return auth.NewPostgresRepository(db).SaveUsers(s.Users.Users) }},
			{"files", func() error { return files.NewPostgresRepository(db).SaveFiles(s.Files.Files) }},
			{"workflow", func() error { return workflow.NewPostgresRepository(db).SaveState(s.Workflow) }},
			{"email", func() error { return emailops.NewPostgresRepository(db).SaveState(s.Email) }},
			{"directory", func() error { return directory.NewPostgresRepository(db).SaveState(s.Directory) }},
			{"rates", func() error { return ratesheets.NewPostgresRepository(db).SaveState(s.Rates) }},
		}
		for _, step := range steps {
			if err := step.save(); err != nil {
				return fmt.Errorf("candidate import %s failed (do not promote): %w", step.name, err)
			}
		}
	}
	results, err := reconcile(db, s)
	if err != nil {
		return err
	}
	_, after, err := loadSource(data)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(hashes, after) {
		return fmt.Errorf("source changed during migration; candidate must not be promoted")
	}
	matched := true
	for _, r := range results {
		matched = matched && r.Match
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string]any{"ok": matched, "sections": results, "sourceSHA256": hashes}); err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("reconciliation mismatch; keep the current live store unchanged")
	}
	return nil
}

func loadSource(dir string) (source, map[string]string, error) {
	var s source
	hashes := map[string]string{}
	for _, input := range []struct {
		name   string
		target any
	}{
		{"auth.users.dev.json", &s.Users}, {"customers.dev.json", &s.Customers}, {"files.dev.json", &s.Files},
		{"workflow.dev.json", &s.Workflow}, {"email.dev.json", &s.Email}, {"directory.dev.json", &s.Directory}, {"shipping-rates.dev.json", &s.Rates},
	} {
		raw, err := os.ReadFile(filepath.Join(dir, input.name))
		if err != nil {
			return s, nil, err
		}
		var header struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return s, nil, fmt.Errorf("invalid %s: %w", input.name, err)
		}
		if header.Version != 1 {
			return s, nil, fmt.Errorf("unsupported snapshot version in %s", input.name)
		}
		if err := json.Unmarshal(raw, input.target); err != nil {
			return s, nil, fmt.Errorf("decode %s: %w", input.name, err)
		}
		sum := sha256.Sum256(raw)
		hashes[input.name] = hex.EncodeToString(sum[:])
	}
	return s, hashes, nil
}

func reconcile(db *sql.DB, s source) ([]result, error) {
	var results []result
	check := func(name string, left, right any) {
		l, r := canonicalRows(left, name), canonicalRows(right, name)
		results = append(results, result{Section: name, SourceCount: len(l), TargetCount: len(r), Match: reflect.DeepEqual(l, r), DifferentFields: differentFields(left, right)})
	}
	u, _, err := auth.NewPostgresRepository(db).LoadUsers()
	if err != nil {
		return nil, err
	}
	check("users", s.Users.Users, u)
	c, requests, _, err := customers.NewPostgresRepository(db).LoadCustomers()
	if err != nil {
		return nil, err
	}
	check("customers", s.Customers.Customers, c)
	check("documentRequests", s.Customers.Requests, requests)
	f, _, err := files.NewPostgresRepository(db).LoadFiles()
	if err != nil {
		return nil, err
	}
	check("files", s.Files.Files, f)
	w, _, err := workflow.NewPostgresRepository(db).LoadState()
	if err != nil {
		return nil, err
	}
	for _, item := range []struct {
		name        string
		left, right any
	}{
		{"products", s.Workflow.Products, w.Products}, {"quotes", s.Workflow.Quotations, w.Quotations}, {"orders", s.Workflow.Manufacturing, w.Manufacturing},
		{"shipping", s.Workflow.Shipping, w.Shipping}, {"payments", s.Workflow.Payments, w.Payments}, {"ledger", s.Workflow.Ledger, w.Ledger},
		{"conversations", s.Workflow.Conversations, w.Conversations}, {"calls", s.Workflow.Calls, w.Calls}, {"callSignals", s.Workflow.CallSignals, w.CallSignals},
		{"rateSheets", s.Workflow.RateSheets, w.RateSheets}, {"notices", s.Workflow.Notices, w.Notices}, {"featured", s.Workflow.Featured, w.Featured},
		{"settings", []any{s.Workflow.Settings}, []any{w.Settings}},
		{"sequence", []any{s.Workflow.LastSequenceDay, s.Workflow.Sequence}, []any{w.LastSequenceDay, w.Sequence}},
	} {
		check(item.name, item.left, item.right)
	}
	e, _, err := emailops.NewPostgresRepository(db).LoadState()
	if err != nil {
		return nil, err
	}
	check("emailSettings", []any{s.Email.Settings}, []any{e.Settings})
	check("emailTemplates", s.Email.Templates, e.Templates)
	check("emailRules", s.Email.Rules, e.Rules)
	check("emailOutbox", s.Email.Outbox, e.Outbox)
	check("emailDeliveries", s.Email.Deliveries, e.Deliveries)
	d, _, err := directory.NewPostgresRepository(db).LoadState()
	if err != nil {
		return nil, err
	}
	check("directoryCategories", s.Directory.Categories, d.Categories)
	check("directoryListings", s.Directory.Listings, d.Listings)
	r, _, err := ratesheets.NewPostgresRepository(db).LoadState()
	if err != nil {
		return nil, err
	}
	check("rateBooks", s.Rates.Books, r.Books)
	return results, nil
}

func differentFields(left, right any) []string {
	index := func(value any) map[string]map[string]any {
		raw, _ := json.Marshal(value)
		var rows []map[string]any
		_ = json.Unmarshal(raw, &rows)
		out := map[string]map[string]any{}
		for _, row := range rows {
			key, _ := row["id"].(string)
			if user, ok := row["user"].(map[string]any); ok {
				key, _ = user["id"].(string)
				row = user
			}
			if entry, ok := row["entryNo"].(string); ok {
				key = entry
			}
			if key != "" {
				out[key] = row
			}
		}
		return out
	}
	l, r := index(left), index(right)
	fields := map[string]bool{}
	for id, row := range l {
		other, ok := r[id]
		if !ok {
			fields["missingRecord"] = true
			continue
		}
		for key, value := range row {
			if !reflect.DeepEqual(normalizeEmpty(value), normalizeEmpty(other[key])) {
				fields[key] = true
			}
		}
		for key := range other {
			if _, ok := row[key]; !ok {
				fields[key] = true
			}
		}
	}
	out := []string{}
	for key := range fields {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func canonicalRows(value any, section string) []string {
	raw, _ := json.Marshal(value)
	var rows []any
	_ = json.Unmarshal(raw, &rows)
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		// Presence is deliberately offline after migration; it is not durable business data.
		if section == "users" {
			if record, ok := row.(map[string]any); ok {
				if user, ok := record["user"].(map[string]any); ok {
					delete(user, "online")
				}
			}
		}
		encoded, _ := json.Marshal(normalizeEmpty(row))
		result = append(result, string(encoded))
	}
	sort.Strings(result)
	return result
}

func normalizeEmpty(value any) any {
	switch v := value.(type) {
	case []any:
		for i := range v {
			v[i] = normalizeEmpty(v[i])
		}
		if len(v) == 0 {
			return nil
		}
		return v
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeEmpty(item)
		}
	}
	return value
}
