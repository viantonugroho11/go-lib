package errors

import "net/http"

// StatusCode maps err's Kind to an HTTP status. Non-*Error inputs return 500.
// Wire this in httpserver's error middleware.
func StatusCode(err error) int {
	switch KindOf(err) {
	case KindValidation:
		return http.StatusBadRequest
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindTooMany:
		return http.StatusTooManyRequests
	case KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
