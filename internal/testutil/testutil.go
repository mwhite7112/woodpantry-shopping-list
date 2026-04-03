//go:build integration

package testutil

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func SetupDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			t.Skipf("postgres testcontainer unavailable: %v", r)
		}
	}()

	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("shopping_list_db"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Skipf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	runMigrations(t, sqlDB)
	return sqlDB
}

func runMigrations(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "db", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".sql" && len(entry.Name()) > 7 && entry.Name()[len(entry.Name())-7:] == ".up.sql" {
			upFiles = append(upFiles, filepath.Join(migrationsDir, entry.Name()))
		}
	}
	sort.Strings(upFiles)

	for _, file := range upFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		if _, err := sqlDB.Exec(string(data)); err != nil {
			t.Fatalf("run migration %s: %v", file, err)
		}
	}
}
