package xlog

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Field helpers for concise and explicit field customization.
func Str(key, val string) Field               { return zap.String(key, val) }
func Bool(key string, val bool) Field         { return zap.Bool(key, val) }
func Int(key string, val int) Field           { return zap.Int(key, val) }
func Int64(key string, val int64) Field       { return zap.Int64(key, val) }
func Float64(key string, val float64) Field   { return zap.Float64(key, val) }
func Time(key string, val time.Time) Field    { return zap.Time(key, val) }
func Dur(key string, val time.Duration) Field { return zap.Duration(key, val) }
func Any(key string, val interface{}) Field   { return zap.Any(key, val) }
func Err(err error) Field                     { return zap.Error(err) }
func NamedError(key string, err error) Field  { return zap.NamedError(key, err) }
func Object(key string, val zapcore.ObjectMarshaler) Field {
	return zap.Object(key, val)
}
func Stringer(key string, val interface{ String() string }) Field {
	return zap.Stringer(key, val)
}
