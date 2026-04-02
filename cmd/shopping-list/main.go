package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/api"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/db"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/logging"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

func main() {
	logging.Setup()

	if err := run(); err != nil {
		slog.Error("shopping-list service failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		return err
	}

	sqlDB, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	if err := runMigrations(sqlDB); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	svc := service.New(service.Config{
		RecipeURL:     cfg.RecipeURL,
		PantryURL:     cfg.PantryURL,
		DictionaryURL: cfg.DictionaryURL,
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("shopping-list service listening", "addr", addr)

	if err := http.ListenAndServe(addr, api.NewRouter(svc)); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

type config struct {
	Port          string
	DBURL         string
	RecipeURL     string
	PantryURL     string
	DictionaryURL string
	LogLevel      string
}

func loadConfigFromEnv() (config, error) {
	cfg := config{
		Port:          envOrDefault("PORT", "8080"),
		DBURL:         os.Getenv("DB_URL"),
		RecipeURL:     os.Getenv("RECIPE_URL"),
		PantryURL:     os.Getenv("PANTRY_URL"),
		DictionaryURL: os.Getenv("DICTIONARY_URL"),
		LogLevel:      envOrDefault("LOG_LEVEL", "info"),
	}

	switch {
	case cfg.DBURL == "":
		return config{}, errors.New("DB_URL is required")
	case cfg.RecipeURL == "":
		return config{}, errors.New("RECIPE_URL is required")
	case cfg.PantryURL == "":
		return config{}, errors.New("PANTRY_URL is required")
	case cfg.DictionaryURL == "":
		return config{}, errors.New("DICTIONARY_URL is required")
	}

	return cfg, nil
}

func runMigrations(sqlDB *sql.DB) error {
	srcDriver, err := iofs.New(db.MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	dbDriver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
