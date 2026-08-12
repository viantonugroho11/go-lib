// Example: load a YAML file into a typed struct, then override via ENV.
//
// Run:
//
//	cd config/example && go run .
//
// Try ENV override:
//
//	EX_APP_PORT=9090 EX_DB_HOST=prod-db go run .
package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	config_load "github.com/viantonugroho11/go-lib/config"
)

type Config struct {
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

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)

	loader := config_load.New(
		"EX", // envPrefix -> EX_APP_PORT, EX_DB_HOST, ...
		"",   // consulKey (skip Consul in this example)
		"",   // consulURL
		config_load.WithConfigFileName("config"),
		config_load.WithConfigFileSearchPaths(dir),
	)

	var cfg Config
	if err := loader.Load(&cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg)
}
