# Changelog

## v0.1.0 - 2026-08-12

Initial release.

- **`Error` type** — stable `Code`, `Kind` for transport mapping, default `Message`, template `Args`, `Cause` for wrap chains. `Is` matches by Code.
- **Kind-specific constructors** — `NewValidation`, `NewNotFound`, `NewConflict`, `NewUnauthorized`, `NewForbidden`, `NewTooMany`, `NewInternal`, `NewUnavailable`.
- **`Resolver` interface** — renders human message; locale-aware via ctx (`ContextWithLocale`, `LocaleFromContext`).
- **`FileResolver`** — external YAML/JSON dictionary directory with per-file locale (`en.yaml`, `id.yaml`, `en.json`). Hot reload via `fsnotify`. Bad reload keeps previous dictionary and fires `WithReloadErrorHook`.
- **Fallback chain** — requested locale → default locale (`WithDefaultLocale`, default `"en"`) → `Error.Message` → `Error.Code`.
- **`text/template` bodies** — `"User {{.id}} not found"` with args passed via `WithArg` / `WithArgs`.
- **Global default resolver** — no-op out of the box; `SetDefaultResolver` wires the app-wide instance; `Resolve(ctx, err)` reads it.
- **HTTP mapping** — `StatusCode(err)` maps `Kind` to HTTP status; ready for `httpserver` error middleware.
- **Tests** — 7 tests, race-clean: kind/code, HTTP mapping, noop fallback, file lookup + fallback chain, hot reload, bad reload keeps previous, global resolver.
