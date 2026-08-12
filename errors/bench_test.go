package errors

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupResolver(b *testing.B) *FileResolver {
	b.Helper()
	dir := b.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "en.yaml"), []byte(`
user.not_found: "User {{.id}} not found"
user.email_taken: "Email {{.email}} already registered"
payment.insufficient: "Balance {{.have}} less than required {{.need}}"
`), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "id.yaml"), []byte(`
user.not_found: "User {{.id}} tidak ditemukan"
`), 0o600)
	r, err := NewFileResolver(dir, WithDefaultLocale("en"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { r.Close() })
	return r
}

func BenchmarkResolve_HotHit(b *testing.B) {
	r := setupResolver(b)
	ctx := ContextWithLocale(context.Background(), "en")
	e := NewNotFound("user.not_found", "def").WithArg("id", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Resolve(ctx, e)
	}
}

func BenchmarkResolve_LocaleFallback(b *testing.B) {
	// "id" has no user.email_taken; falls back to "en".
	r := setupResolver(b)
	ctx := ContextWithLocale(context.Background(), "id")
	e := NewConflict("user.email_taken", "def").WithArg("email", "a@b.co")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Resolve(ctx, e)
	}
}

func BenchmarkResolve_MissingCode(b *testing.B) {
	// Not in any dict; fallback chain terminates at Error.Message.
	r := setupResolver(b)
	ctx := ContextWithLocale(context.Background(), "en")
	e := NewInternal("boot.database", "database unreachable")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Resolve(ctx, e)
	}
}

func BenchmarkStatusCode(b *testing.B) {
	e := NewNotFound("x", "y")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StatusCode(e)
	}
}

func BenchmarkGlobalResolve(b *testing.B) {
	r := setupResolver(b)
	SetDefaultResolver(r)
	b.Cleanup(func() { SetDefaultResolver(nil) })
	ctx := ContextWithLocale(context.Background(), "en")
	e := NewNotFound("user.not_found", "def").WithArg("id", 7)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Resolve(ctx, e)
	}
}
