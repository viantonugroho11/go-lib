package config_load

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type appConfig struct {
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
	Database struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"database"`
}

func writeTempYAML(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed writing temp yaml: %v", err)
	}
	return path
}

func TestLoad_InvalidInput_Nil(t *testing.T) {
	loader := New("APP", "", "")
	if err := loader.Load(nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestLoad_InvalidInput_NonStructPointer(t *testing.T) {
	loader := New("APP", "", "")
	var n int
	if err := loader.Load(&n); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestLoad_FileAndEnv_Success(t *testing.T) {
	t.Setenv("APP_DATABASE_HOST", "127.0.0.1")
	dir := t.TempDir()
	_ = writeTempYAML(t, dir, "config.yaml", `
app:
  name: "myapp"
database:
  host: "localhost"
  port: 5432
`)

	loader := New("APP", "", "", WithConfigFileSearchPaths(dir), WithStructTagName("mapstructure"))
	var cfg appConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.Name != "myapp" {
		t.Fatalf("expected App.Name=myapp, got %q", cfg.App.Name)
	}
	if cfg.Database.Port != 5432 {
		t.Fatalf("expected Database.Port=5432, got %d", cfg.Database.Port)
	}
	if cfg.Database.Host != "127.0.0.1" {
		t.Fatalf("expected Database.Host overridden by env to 127.0.0.1, got %q", cfg.Database.Host)
	}
}

func TestLoad_FileNotFound_Error(t *testing.T) {
	dir := t.TempDir()
	loader := New("APP", "", "", WithConfigFileSearchPaths(dir))
	var cfg appConfig
	err := loader.Load(&cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrConfigFileNotFound) {
		t.Fatalf("expected ErrConfigFileNotFound, got: %v", err)
	}
}

func TestLoad_ConsulFallbackToFile_Success(t *testing.T) {
	// Prepare a valid local file for fallback
	dir := t.TempDir()
	_ = writeTempYAML(t, dir, "config.yaml", `
app:
  name: "fromfile"
database:
  host: "fallback-host"
  port: 3306
`)

	// Set an env override to verify env still applied after fallback
	t.Setenv("APP_DATABASE_HOST", "env-host")

	// Use an invalid consul endpoint and limit attempts to 1 to keep test fast
	loader := New("APP", "dummy/key", "127.0.0.1:1",
		WithLoadFromConsulMaxAttempt(1),
		WithStructTagName("mapstructure"),
		WithConfigFileSearchPaths(dir),
	)

	var cfg appConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.Name != "fromfile" {
		t.Fatalf("expected App.Name=fromfile, got %q", cfg.App.Name)
	}
	if cfg.Database.Port != 3306 {
		t.Fatalf("expected Database.Port=3306, got %d", cfg.Database.Port)
	}
	if cfg.Database.Host != "env-host" {
		t.Fatalf("expected Database.Host overridden by env to env-host, got %q", cfg.Database.Host)
	}
}


