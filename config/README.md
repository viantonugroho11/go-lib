## config

A small configuration loader built on top of Viper. It can read configuration from Consul KV (YAML), fall back to local config files, and apply environment variable overrides (in file mode). It uses a simple options pattern and supports custom struct tag names.

- **Consul first, then file+env fallback**
- **Environment variable overrides in file mode**
- **Custom struct tag name (json/mapstructure)**
- **Config search paths and file name**

### Installation

- Import according to your module path (in this repo the module is named `config`).

### Quickstart (File + Env overrides)

1) Create a file named `config.yaml` in one of your search paths (default is current directory `.`):

```yaml
app:
  name: "myapp"
database:
  host: "localhost"
  port: 5432
```

2) Load it in your Go program; environment variables with the chosen prefix can override file values:

```go
package main

import (
	"fmt"

	config_load "config"
)

type AppConfig struct {
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
	Database struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"database"`
}

func main() {
	loader := config_load.New(
		"APP", // env prefix
		"",    // consul key (empty => skip consul)
		"",    // consul address (empty => skip consul)
		config_load.WithStructTagName("mapstructure"),
		config_load.WithConfigFileSearchPaths("."),
		// config_load.WithConfigFileName("config"), // default is "config"
	)

	var cfg AppConfig
	if err := loader.Load(&cfg); err != nil {
		panic(err)
	}

	fmt.Println(cfg.App.Name, cfg.Database.Host, cfg.Database.Port)
}
```

3) Override via environment variable (uses prefix and dot→underscore rules):

```bash
export APP_DATABASE_HOST=127.0.0.1
go run .
```

Notes:
- The file name defaults to `config`, and Viper infers the type from the extension. Place `config.yaml` / `config.yml` / `config.json` / `config.toml` in any configured search path.
- The environment key replacer maps `.` to `_`, so `database.host` becomes `APP_DATABASE_HOST`.

### Loading from Consul KV (YAML)

Put your YAML under a Consul KV key, then configure the loader with a Consul address and key. If reading from Consul fails, it will fall back to file+env mode.

```go
loader := config_load.New(
	"APP",
	"my/app/config",  // Consul KV key
	"127.0.0.1:8500", // Consul address (host:port)
	config_load.WithLoadFromConsulMaxAttempt(5),
	config_load.WithStructTagName("mapstructure"),
)

var cfg AppConfig
if err := loader.Load(&cfg); err != nil {
	// If Consul fails, loader attempts file+env
	panic(err)
}
```

Important behavior:
- The Consul value is parsed as YAML.
- If Consul load succeeds, environment variables are NOT applied on top.
- If Consul load fails, the loader falls back to local file; in this fallback mode, environment variables DO override file values.

### Options

- `WithConfigFileSearchPaths(paths ...string)`: Add directories to search for `config.<ext>`
- `WithConfigFileName(name string)`: Change the base name (default: `config`)
- `WithStructTagName(name string)`: Decoder tag to use (default: `json`; often you’ll want `mapstructure`)
- `WithLoadFromConsulMaxAttempt(n int)`: Max retry attempts when reading from Consul (default: `5`)

### Errors

- `ErrInvalidInput`: The provided `cfg` must be a non-nil pointer to a struct
- `ErrConfigFileNotFound`: No `config` file found in the configured search paths (when running in file mode)

### Environment Variables

- Prefix is provided via the first argument to `New` (e.g., `"APP"`)
- Keys use dot notation in struct tags and are mapped to env vars by replacing `.` with `_`
  - Example: `database.host` → `APP_DATABASE_HOST`

### Requirements

- Go 1.23+

### Changelog

See `CHANGELOG.md` for release notes.
