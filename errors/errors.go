// Package errors provides typed application errors with a stable Code, a Kind for
// transport mapping (HTTP status, gRPC code), and configurable human messages resolved
// per-locale via an external file dictionary.
//
// The invariants:
//   - Code is stable and machine-facing. Clients switch on Code, never on Message.
//   - Kind determines transport status; it is set in code, not configuration.
//   - Message and its locale variants are configuration; edit the dictionary,
//     not the code.
//
// Usage:
//
//	err := errors.NewNotFound("user.not_found", "User not found").
//	    WithArgs(map[string]any{"id": userID}).
//	    Wrap(pgErr)
//	// In HTTP middleware:
//	msg := errors.Resolve(ctx, err)                 // locale-aware
//	status := errors.StatusCode(err)                // Kind -> HTTP status
package errors

import (
	stderrors "errors"
	"fmt"
)

// Kind classifies an Error for transport mapping. Set in code; do not derive from configuration.
type Kind int

const (
	KindUnknown      Kind = iota
	KindValidation        // 400 — client sent bad input
	KindUnauthorized      // 401 — no or invalid credentials
	KindForbidden         // 403 — authenticated but not allowed
	KindNotFound          // 404 — resource missing
	KindConflict          // 409 — precondition / duplicate
	KindTooMany           // 429 — rate limited
	KindInternal          // 500 — server bug
	KindUnavailable       // 503 — upstream down / degraded
)

func (k Kind) String() string {
	switch k {
	case KindValidation:
		return "validation"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindTooMany:
		return "too_many"
	case KindInternal:
		return "internal"
	case KindUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// Error is the typed application error. Only Code + Kind are load-bearing for behavior;
// Message/Args are inputs to the resolver for human display.
type Error struct {
	Code    string         // stable identifier, e.g. "user.not_found"
	Kind    Kind           // transport class
	Message string         // default human text; used when no resolver entry matches
	Args    map[string]any // template variables for the dictionary entry
	Cause   error          // wrapped underlying error
}

// Error implements the error interface using the DEFAULT message. For a locale-aware,
// dictionary-resolved message use Resolve(ctx, err).
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.defaultMessage(), e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.defaultMessage())
}

// Unwrap returns the wrapped cause for errors.Is / errors.As chains.
func (e *Error) Unwrap() error { return e.Cause }

// Is matches by Code; two Errors are the same if they carry the same Code.
func (e *Error) Is(target error) bool {
	var t *Error
	if !stderrors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

func (e *Error) defaultMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// New constructs an Error with the given kind, code, and default message.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Kind-specific constructors — sugar for readability at call sites.

func NewValidation(code, msg string) *Error   { return New(KindValidation, code, msg) }
func NewUnauthorized(code, msg string) *Error { return New(KindUnauthorized, code, msg) }
func NewForbidden(code, msg string) *Error    { return New(KindForbidden, code, msg) }
func NewNotFound(code, msg string) *Error     { return New(KindNotFound, code, msg) }
func NewConflict(code, msg string) *Error     { return New(KindConflict, code, msg) }
func NewTooMany(code, msg string) *Error      { return New(KindTooMany, code, msg) }
func NewInternal(code, msg string) *Error     { return New(KindInternal, code, msg) }
func NewUnavailable(code, msg string) *Error  { return New(KindUnavailable, code, msg) }

// WithArgs sets template variables for the dictionary lookup. Chainable.
func (e *Error) WithArgs(args map[string]any) *Error {
	e.Args = args
	return e
}

// WithArg is a single-key convenience over WithArgs.
func (e *Error) WithArg(key string, value any) *Error {
	if e.Args == nil {
		e.Args = make(map[string]any, 1)
	}
	e.Args[key] = value
	return e
}

// Wrap attaches an underlying cause. Chainable.
func (e *Error) Wrap(cause error) *Error {
	e.Cause = cause
	return e
}

// As returns the *Error if err (or anything it wraps) is one, else nil.
func As(err error) *Error {
	var e *Error
	if stderrors.As(err, &e) {
		return e
	}
	return nil
}

// KindOf returns the Kind of err, or KindUnknown when err is not an *Error.
func KindOf(err error) Kind {
	if e := As(err); e != nil {
		return e.Kind
	}
	return KindUnknown
}

// CodeOf returns the Code of err, or "" when err is not an *Error.
func CodeOf(err error) string {
	if e := As(err); e != nil {
		return e.Code
	}
	return ""
}
