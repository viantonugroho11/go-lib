package config_load

import (
	"os"
	"path/filepath"
	"testing"
)

type benchCfg struct {
	App struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	} `json:"app"`
	DB struct {
		Host string `json:"host"`
		Port int    `json:"port"`
		User string `json:"user"`
	} `json:"db"`
	Kafka struct {
		Brokers []string `json:"brokers"`
	} `json:"kafka"`
}

func setupBenchFile(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	body := []byte(`app:
  name: bench
  port: 8080
db:
  host: localhost
  port: 5432
  user: app
kafka:
  brokers: ["kafka-1:9092", "kafka-2:9092"]
`)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		b.Fatal(err)
	}
	return dir
}

func BenchmarkLoadFromFile(b *testing.B) {
	dir := setupBenchFile(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader := New("BENCH", "", "",
			WithConfigFileName("config"),
			WithConfigFileSearchPaths(dir),
		)
		var cfg benchCfg
		if err := loader.Load(&cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadWithEnvOverride(b *testing.B) {
	dir := setupBenchFile(b)
	b.Setenv("BENCH_APP_PORT", "9090")
	b.Setenv("BENCH_DB_HOST", "prod-db")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader := New("BENCH", "", "",
			WithConfigFileName("config"),
			WithConfigFileSearchPaths(dir),
		)
		var cfg benchCfg
		if err := loader.Load(&cfg); err != nil {
			b.Fatal(err)
		}
	}
}
