package otel

import (
	"crypto/tls"

	"google.golang.org/grpc/credentials"
)

// credsFromTLS wraps a *tls.Config for gRPC transport credentials.
func credsFromTLS(cfg *tls.Config) credentials.TransportCredentials {
	return credentials.NewTLS(cfg)
}
