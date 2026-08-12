package errors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// FileResolver watches a directory of dictionary files (one per locale) and resolves
// Error messages against the corresponding compiled templates.
//
// File conventions:
//   - Filename base = locale. "en.yaml" -> "en", "id.json" -> "id".
//   - Supported extensions: .yaml, .yml, .json.
//   - File body is a flat map: code -> template string (text/template syntax, e.g. "User {{.id}} not found").
//
// Fallback chain per lookup:
//   1. requested locale (LocaleFromContext or the resolver's LocaleFunc)
//   2. default locale (WithDefaultLocale, default "en")
//   3. Error.Message
//   4. Error.Code
//
// Hot reload: any Write/Create/Rename event on a file re-parses that locale.
// A parse error on reload keeps the previous dictionary in place and calls the ErrorHook.
type FileResolver struct {
	dir            string
	defaultLocale  string
	localeFunc     LocaleFunc
	errorHook      func(err error)
	dicts          atomic.Value // map[string]dict
	watcher        *fsnotify.Watcher
	watchClosed    chan struct{}
	watchOnce      sync.Once
	watchStopOnce  sync.Once
}

type dict struct {
	templates map[string]*template.Template
}

// ResolverOption configures a FileResolver.
type ResolverOption func(*FileResolver)

// WithDefaultLocale sets the fallback locale (default "en").
func WithDefaultLocale(locale string) ResolverOption {
	return func(r *FileResolver) { r.defaultLocale = locale }
}

// WithLocaleFunc overrides how the resolver reads locale from ctx.
// Default: LocaleFromContext.
func WithLocaleFunc(fn LocaleFunc) ResolverOption {
	return func(r *FileResolver) {
		if fn != nil {
			r.localeFunc = fn
		}
	}
}

// WithReloadErrorHook installs a callback fired when a hot-reload parse fails.
// The previous dictionary stays active; use this to log the failure.
func WithReloadErrorHook(fn func(err error)) ResolverOption {
	return func(r *FileResolver) { r.errorHook = fn }
}

// NewFileResolver loads every supported file in dir and starts a watcher for hot reload.
// Returns an error if the initial load fails; individual reload failures during runtime
// are surfaced via the ErrorHook.
func NewFileResolver(dir string, opts ...ResolverOption) (*FileResolver, error) {
	r := &FileResolver{
		dir:           dir,
		defaultLocale: "en",
		localeFunc:    LocaleFromContext,
		watchClosed:   make(chan struct{}),
	}
	for _, o := range opts {
		o(r)
	}
	dicts, err := loadAll(dir)
	if err != nil {
		return nil, fmt.Errorf("errors: load dictionaries from %s: %w", dir, err)
	}
	r.dicts.Store(dicts)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("errors: create watcher: %w", err)
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("errors: watch %s: %w", dir, err)
	}
	r.watcher = watcher
	go r.watchLoop()
	return r, nil
}

// Resolve renders e's message against the dictionary. Fallback chain applies.
func (r *FileResolver) Resolve(ctx context.Context, e *Error) string {
	if e == nil {
		return ""
	}
	dicts := r.currentDicts()
	locale := r.localeFunc(ctx)

	if msg, ok := r.render(dicts, locale, e); ok {
		return msg
	}
	if locale != r.defaultLocale {
		if msg, ok := r.render(dicts, r.defaultLocale, e); ok {
			return msg
		}
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// Close stops the file watcher. Safe to call multiple times.
func (r *FileResolver) Close() error {
	var err error
	r.watchStopOnce.Do(func() {
		err = r.watcher.Close()
		<-r.watchClosed
	})
	return err
}

func (r *FileResolver) render(dicts map[string]dict, locale string, e *Error) (string, bool) {
	if locale == "" {
		return "", false
	}
	d, ok := dicts[locale]
	if !ok {
		return "", false
	}
	tmpl, ok := d.templates[e.Code]
	if !ok {
		return "", false
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.Args); err != nil {
		return "", false
	}
	return buf.String(), true
}

func (r *FileResolver) currentDicts() map[string]dict {
	if v, ok := r.dicts.Load().(map[string]dict); ok {
		return v
	}
	return nil
}

func (r *FileResolver) watchLoop() {
	defer close(r.watchClosed)
	for {
		select {
		case ev, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if !isSupported(ev.Name) {
				continue
			}
			r.reload(ev.Name)
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			if r.errorHook != nil {
				r.errorHook(err)
			}
		}
	}
}

// reload re-parses a single file and swaps the affected locale into the atomic map.
// On parse error, the previous dictionary stays put and errorHook is fired.
func (r *FileResolver) reload(path string) {
	locale := localeFromFilename(path)
	if locale == "" {
		return
	}
	d, err := loadFile(path)
	if err != nil {
		if r.errorHook != nil {
			r.errorHook(fmt.Errorf("errors: reload %s: %w", path, err))
		}
		return
	}
	// atomic swap: build a fresh outer map so readers never see a partial write.
	prev := r.currentDicts()
	next := make(map[string]dict, len(prev)+1)
	for k, v := range prev {
		next[k] = v
	}
	next[locale] = d
	r.dicts.Store(next)
}

// --- file I/O ---

func loadAll(dir string) (map[string]dict, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]dict, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		if !isSupported(path) {
			continue
		}
		locale := localeFromFilename(path)
		if locale == "" {
			continue
		}
		d, err := loadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		out[locale] = d
	}
	return out, nil
}

func loadFile(path string) (dict, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dict{}, err
	}
	raw := make(map[string]string)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return dict{}, fmt.Errorf("yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &raw); err != nil {
			return dict{}, fmt.Errorf("json: %w", err)
		}
	default:
		return dict{}, fmt.Errorf("unsupported extension: %s", ext)
	}
	templates := make(map[string]*template.Template, len(raw))
	for code, tmplStr := range raw {
		t, err := template.New(code).Option("missingkey=zero").Parse(tmplStr)
		if err != nil {
			return dict{}, fmt.Errorf("parse template %s: %w", code, err)
		}
		templates[code] = t
	}
	return dict{templates: templates}, nil
}

func isSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

func localeFromFilename(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return ""
	}
	return strings.TrimSuffix(base, ext)
}
