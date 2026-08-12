package errors

import (
	"context"
	"sync/atomic"
)

// Resolver renders the human message for an Error. Implementations are locale-aware
// via ctx (see LocaleFromContext / LocaleFunc). Return "" to let the caller fall back
// to Error.Message.
type Resolver interface {
	Resolve(ctx context.Context, e *Error) string
}

// LocaleFunc extracts the caller's locale (e.g. "en", "id") from ctx.
// Default: LocaleFromContext (ctxKey lookup); override via WithLocaleFunc.
type LocaleFunc func(ctx context.Context) string

type ctxKey struct{}

// ContextWithLocale returns ctx with the locale stored under the package's default key.
// Use in HTTP middleware after parsing Accept-Language, or wherever locale is known.
func ContextWithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, ctxKey{}, locale)
}

// LocaleFromContext returns the locale stored in ctx, or "" if none.
func LocaleFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

// --- Global default resolver ---
//
// Business code calls errors.Resolve(ctx, err) without threading a Resolver through
// every layer. A no-op resolver (returns Error.Message) is installed by default so
// the package works out of the box; boot code replaces it via SetDefaultResolver.

// resolverBox lets us atomic.Value.Store different concrete Resolver implementations
// under a single struct type. atomic.Value requires the stored concrete type to be
// consistent across Stores — an interface value alone does not satisfy that.
type resolverBox struct{ r Resolver }

var defaultResolver atomic.Value // holds resolverBox

func init() {
	defaultResolver.Store(resolverBox{r: noopResolver{}})
}

// SetDefaultResolver installs the package-wide resolver. Safe to call at boot; concurrent
// with Resolve, atomic swap ensures no torn read. Do not swap resolvers on the hot path.
func SetDefaultResolver(r Resolver) {
	if r == nil {
		r = noopResolver{}
	}
	defaultResolver.Store(resolverBox{r: r})
}

// DefaultResolver returns the currently installed resolver.
func DefaultResolver() Resolver {
	if v, ok := defaultResolver.Load().(resolverBox); ok {
		return v.r
	}
	return noopResolver{}
}

// Resolve renders err through the default resolver. Returns "" when err is not an *Error;
// callers should fall back to err.Error() in that case.
func Resolve(ctx context.Context, err error) string {
	e := As(err)
	if e == nil {
		return ""
	}
	if msg := DefaultResolver().Resolve(ctx, e); msg != "" {
		return msg
	}
	return e.defaultMessage()
}

// noopResolver returns the Error.Message as-is. Used until SetDefaultResolver is called.
type noopResolver struct{}

func (noopResolver) Resolve(_ context.Context, e *Error) string {
	if e == nil {
		return ""
	}
	return e.defaultMessage()
}
