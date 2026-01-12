package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/throw-if-null/molecular/internal/api"
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
	defer db.Close()

	s := store.New(db)
	if err := s.Init(); err != nil {
		log.Fatalf("failed to init schema: %v", err)
	}

	srv := silicon.NewServer(s)
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
