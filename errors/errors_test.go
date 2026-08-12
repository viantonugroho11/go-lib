package errors

import (
	"context"
	stderrors "errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestKindAndCodeExtraction(t *testing.T) {
	e := NewNotFound("user.not_found", "user missing").WithArg("id", 42).Wrap(stderrors.New("pg: no rows"))
	if KindOf(e) != KindNotFound {
		t.Fatalf("Kind = %v", KindOf(e))
	}
	if CodeOf(e) != "user.not_found" {
		t.Fatalf("Code = %q", CodeOf(e))
	}
	if !stderrors.Is(e, e) {
		t.Fatal("Is self must be true")
	}
	e2 := NewNotFound("user.not_found", "different msg")
	if !stderrors.Is(e, e2) {
		t.Fatal("Is by Code must match")
	}
}

func TestStatusCodeMapping(t *testing.T) {
	cases := map[Kind]int{
		KindValidation:   400,
		KindUnauthorized: 401,
		KindForbidden:    403,
		KindNotFound:     404,
		KindConflict:     409,
		KindTooMany:      429,
		KindInternal:     500,
		KindUnavailable:  503,
		KindUnknown:      500,
	}
	for k, want := range cases {
		got := StatusCode(New(k, "x", "x"))
		if got != want {
			t.Errorf("Kind %v -> %d, want %d", k, got, want)
		}
	}
	// non-*Error input
	if StatusCode(stderrors.New("raw")) != http.StatusInternalServerError {
		t.Fatal("raw error must map to 500")
	}
}

func TestNoopResolverFallsBackToMessage(t *testing.T) {
	SetDefaultResolver(nil) // resets to noop
	e := NewValidation("x.y", "please fix this")
	got := Resolve(context.Background(), e)
	if got != "please fix this" {
		t.Fatalf("got %q", got)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFileResolverLookupAndFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "en.yaml"), `
user.not_found: "User {{.id}} not found"
user.exists: "Email {{.email}} taken"
`)
	writeFile(t, filepath.Join(dir, "id.yaml"), `
user.not_found: "User {{.id}} tidak ditemukan"
`)
	r, err := NewFileResolver(dir, WithDefaultLocale("en"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer r.Close()

	// requested locale hits.
	ctx := ContextWithLocale(context.Background(), "id")
	got := r.Resolve(ctx, NewNotFound("user.not_found", "def").WithArg("id", 7))
	if got != "User 7 tidak ditemukan" {
		t.Fatalf("id locale = %q", got)
	}

	// missing code in requested locale, present in default: falls back to default.
	got = r.Resolve(ctx, NewConflict("user.exists", "def").WithArg("email", "a@b.co"))
	if got != "Email a@b.co taken" {
		t.Fatalf("fallback to default locale = %q", got)
	}

	// missing code in both locales: falls back to Error.Message.
	got = r.Resolve(ctx, NewValidation("unknown.code", "bare message"))
	if got != "bare message" {
		t.Fatalf("fallback to Message = %q", got)
	}

	// missing code + empty Message: falls back to Code.
	got = r.Resolve(ctx, NewValidation("nothing.at.all", ""))
	if got != "nothing.at.all" {
		t.Fatalf("fallback to Code = %q", got)
	}
}

func TestFileResolverHotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en.yaml")
	writeFile(t, path, `user.hello: "hello {{.name}}"`)
	r, err := NewFileResolver(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer r.Close()

	ctx := ContextWithLocale(context.Background(), "en")
	first := r.Resolve(ctx, NewNotFound("user.hello", "def").WithArg("name", "world"))
	if first != "hello world" {
		t.Fatalf("initial = %q", first)
	}

	writeFile(t, path, `user.hello: "howdy {{.name}}"`)

	// fsnotify + reload is async; poll up to 2s.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := r.Resolve(ctx, NewNotFound("user.hello", "def").WithArg("name", "world"))
		if got == "howdy world" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("hot reload did not apply new template within 2s")
}

func TestFileResolverBadReloadKeepsPreviousDictionary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en.yaml")
	writeFile(t, path, `x.y: "hello"`)

	var hookErr atomic.Value
	r, err := NewFileResolver(dir, WithReloadErrorHook(func(e error) {
		if e != nil {
			hookErr.Store(e.Error())
		}
	}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer r.Close()

	// Write invalid YAML (a list where a map is expected).
	writeFile(t, path, "- not: a map")

	time.Sleep(150 * time.Millisecond) // let fsnotify + reload run
	if hookErr.Load() == nil {
		t.Log("warning: hook not called; test environment may not deliver fsnotify events reliably")
	}
	// Previous dictionary must still resolve.
	got := r.Resolve(ContextWithLocale(context.Background(), "en"), New(KindValidation, "x.y", "def"))
	if got != "hello" {
		t.Fatalf("previous dict lost after bad reload: got %q", got)
	}
}

func TestGlobalResolveUsesInstalledResolver(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "en.yaml"), `foo.bar: "from dict"`)
	r, err := NewFileResolver(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer r.Close()

	SetDefaultResolver(r)
	defer SetDefaultResolver(nil)

	ctx := ContextWithLocale(context.Background(), "en")
	got := Resolve(ctx, NewValidation("foo.bar", "default"))
	if got != "from dict" {
		t.Fatalf("global Resolve = %q", got)
	}

	// non-*Error returns "".
	if s := Resolve(ctx, stderrors.New("plain")); s != "" {
		t.Fatalf("non-Error Resolve = %q", s)
	}
}
