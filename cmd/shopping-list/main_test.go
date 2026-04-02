package main

import "testing"

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DB_URL", "postgres://shopping:secret@localhost/shopping_list_db?sslmode=disable")
	t.Setenv("RECIPE_URL", "http://recipes")
	t.Setenv("PANTRY_URL", "http://pantry")
	t.Setenv("DICTIONARY_URL", "http://ingredients")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("loadConfigFromEnv() error = %v", err)
	}

	if cfg.Port != "9090" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.DBURL == "" || cfg.RecipeURL == "" || cfg.PantryURL == "" || cfg.DictionaryURL == "" {
		t.Fatalf("expected required URLs to be populated: %+v", cfg)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadConfigFromEnvMissingRequired(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("LOG_LEVEL", "info")

	if _, err := loadConfigFromEnv(); err == nil {
		t.Fatal("expected error for missing required env vars")
	}
}
