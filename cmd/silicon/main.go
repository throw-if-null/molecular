package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/config"
	"github.com/throw-if-null/molecular/internal/lithium"
	"github.com/throw-if-null/molecular/internal/silicon"
	"github.com/throw-if-null/molecular/internal/store"
	"github.com/throw-if-null/molecular/internal/version"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath, err := ensureDBPath()
	if err != nil {
		log.Fatalf("failed to prepare db path: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("failed to open sqlite db: %v", err)
	}
	// configure busy timeout to reduce SQLITE_BUSY transient failures
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)
	defer db.Close()

	s := store.New(db)
	if err := s.Init(); err != nil {
		log.Fatalf("failed to init schema: %v", err)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %v", err)
	}

	cfgRes := config.Load(repoRoot)
	if cfgRes.ParseError != nil {
		log.Fatalf("failed to parse config at %s: %v", cfgRes.Path, cfgRes.ParseError)
	}

	poll := time.Duration(cfgRes.Config.Silicon.PollIntervalMS) * time.Millisecond
	ctx := context.Background()
	// Reconcile any in-flight attempts from previous runs before starting workers.
	if err := s.ReconcileInFlightAttempts(repoRoot); err != nil {
		log.Printf("warning: reconcile in-flight attempts failed: %v", err)
	}
	lithCancel := silicon.StartLithiumWorker(ctx, s, repoRoot, &lithium.RealExecRunner{}, poll)
	defer lithCancel()
	carbonCancel := silicon.StartCarbonWorker(ctx, s, repoRoot, &silicon.RealCommandRunner{}, cfgRes.Config.Workers.CarbonCommand, poll)
	defer carbonCancel()
	heliumCancel := silicon.StartHeliumWorker(ctx, s, repoRoot, &silicon.RealCommandRunner{}, cfgRes.Config.Workers.HeliumCommand, poll)
	defer heliumCancel()
	chlorineCancel := silicon.StartChlorineWorker(ctx, s, repoRoot, poll)
	defer chlorineCancel()

	srv := silicon.NewServer(s, cfgRes.Config.Retry.CarbonBudget, cfgRes.Config.Retry.HeliumBudget, cfgRes.Config.Retry.ReviewBudget)
	addr := fmt.Sprintf("%s:%d", api.DefaultHost, api.DefaultPort)
	log.Printf("silicon %s (%s) listening on http://%s", version.Version, version.Commit, addr)
	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
}

func ensureDBPath() (string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(repoRoot, ".molecular")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "molecular.db"), nil
}
