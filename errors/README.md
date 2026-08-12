## errors

Typed application errors with stable `Code`, `Kind` for transport mapping (HTTP status, gRPC code), and configurable human messages resolved per-locale via an external file dictionary with hot reload.

### Design invariant

- `Code` + `Kind` live in **code**. Never configurable. Clients switch on `Code`, never on `Message`.
- `Message` and locale variants live in **configuration** (YAML/JSON). Ops edit the dictionary, not Go source.

### Quick Start

```go
// boot
resolver, _ := errors.NewFileResolver("./messages",
    errors.WithDefaultLocale("en"),
    errors.WithReloadErrorHook(func(err error) {
        xlog.Logger().Error("dict reload", xlog.Err(err))
    }),
)
defer resolver.Close()
errors.SetDefaultResolver(resolver)

// business
if user == nil {
    return errors.NewNotFound("user.not_found", "User not found").WithArg("id", userID)
}

// HTTP middleware
ctx := errors.ContextWithLocale(r.Context(), "id")
w.WriteHeader(errors.StatusCode(err))
json.NewEncoder(w).Encode(map[string]string{
    "code":    errors.CodeOf(err),
    "message": errors.Resolve(ctx, err),
})
```

### Dictionary files

Directory of `<locale>.(yaml|yml|json)`; body is a flat map `code → template`. `text/template` syntax, `missingkey=zero`.

`messages/en.yaml`
```yaml
user.not_found: "User {{.id}} not found"
user.email_taken: "Email {{.email}} already registered"
payment.insufficient: "Balance {{.have}} less than required {{.need}}"
```

`messages/id.yaml`
```yaml
user.not_found: "User {{.id}} tidak ditemukan"
user.email_taken: "Email {{.email}} sudah terdaftar"
```

Fallback chain per lookup: **requested locale → default locale → `Error.Message` → `Error.Code`**.

### Kinds and HTTP mapping

| Kind | HTTP | Constructor |
|------|------|-------------|
| `KindValidation` | 400 | `NewValidation` |
| `KindUnauthorized` | 401 | `NewUnauthorized` |
| `KindForbidden` | 403 | `NewForbidden` |
| `KindNotFound` | 404 | `NewNotFound` |
| `KindConflict` | 409 | `NewConflict` |
| `KindTooMany` | 429 | `NewTooMany` |
| `KindInternal` | 500 | `NewInternal` |
| `KindUnavailable` | 503 | `NewUnavailable` |

### Options

`FileResolver`:
- `WithDefaultLocale(locale)` — fallback locale (default `"en"`).
- `WithLocaleFunc(fn)` — custom locale extraction from ctx.
- `WithReloadErrorHook(fn)` — log/report reload parse failures.

### Wrap chain

```go
if err := pg.QueryRow(...).Scan(&u); err != nil {
    return errors.NewInternal("user.lookup_failed", "").Wrap(err)
}
// errors.Is / errors.As work; Is matches by Code.
```

### Benchmark

Apple M2 (arm64):

```
BenchmarkResolve_HotHit            ~257 ns/op       ~296 B/op   ~7 allocs/op
BenchmarkResolve_LocaleFallback    ~281 ns/op       ~304 B/op   ~7 allocs/op
BenchmarkResolve_MissingCode        ~25 ns/op          0 B/op    0 allocs/op
BenchmarkStatusCode                  ~46 ns/op          8 B/op    1 allocs/op
BenchmarkGlobalResolve             ~321 ns/op       ~296 B/op   ~8 allocs/op
```

Hot-hit path dominated by `text/template` execution + `bytes.Buffer` alloc; ~4M ops/sec/core.

### Wire with `httpserver`

Sample middleware — turn `*Error` into JSON with locale-aware body:

```go
func ErrorMiddleware(h func(w http.ResponseWriter, r *http.Request) error) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            ctx := errors.ContextWithLocale(r.Context(), parseAcceptLang(r))
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(errors.StatusCode(err))
            _ = json.NewEncoder(w).Encode(map[string]string{
                "code":    errors.CodeOf(err),
                "message": errors.Resolve(ctx, err),
            })
        }
    })
}
```
