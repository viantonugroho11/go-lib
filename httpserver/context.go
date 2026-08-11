package httpserver

type ctxKey string

// CtxKeyRequestID is the default context key used by the request-ID middleware.
const CtxKeyRequestID ctxKey = "httpserver.request_id"

// RequestIDFromContext returns the request ID stored by the middleware, or "".
func RequestIDFromContext(ctx interface{ Value(any) any }) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(CtxKeyRequestID).(string); ok {
		return v
	}
	return ""
}
