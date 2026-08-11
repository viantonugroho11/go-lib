
## config/v0.1.3 - 2026-08-11

### Security

- **deps:** bump `google.golang.org/grpc` to v1.83.0 (fixes CVE < 1.79.3, < 1.82.1)
- **deps:** bump `golang.org/x/crypto` to v0.54.0 (fixes multiple CVE < 0.45.0, < 0.52.0)
- **deps:** bump `golang.org/x/net` to v0.57.0 (fixes CVE < 0.55.0)
- **deps:** bump `go.opentelemetry.io/otel` to v1.45.0 (fixes CVE 1.36.0–1.40.0)

### Breaking

- Go directive raised to 1.25.0 (required by upgraded grpc / otel).


## config/v0.1.2 - 2026-07-16

### Bug Fixes

- **config:** preserve consul error when fallback to file also fails — previously consul error was silently discarded
- **config:** fix typo "consule" in log message


## config/v0.1.1 - 2026-03-06

### Changes

- update usage for import client
- **config:** update example config


## config/v0.1.0 - 2026-01-03

### Changes

- **config:** update changelog for config/v0.1.0
- **config:** fixed formatting
- **config:** add library config

