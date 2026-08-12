// Example: typed errors + external dictionary + locale-aware resolve.
//
// Run:
//
//	cd errors/example && go run .
package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/viantonugroho11/go-lib/errors"
)

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "messages")

	resolver, err := errors.NewFileResolver(dir,
		errors.WithDefaultLocale("en"),
		errors.WithReloadErrorHook(func(e error) {
			log.Printf("dict reload error: %v", e)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resolver.Close()
	errors.SetDefaultResolver(resolver)

	// --- business layer produces errors with stable codes ---
	notFound := errors.NewNotFound("user.not_found", "User not found").WithArg("id", 42)
	insufficient := errors.NewValidation("payment.insufficient", "").
		WithArgs(map[string]any{"have": 100, "need": 250})
	unknownCode := errors.NewInternal("boot.database", "database unreachable")

	// --- resolve in different locales ---
	for _, locale := range []string{"en", "id"} {
		ctx := errors.ContextWithLocale(context.Background(), locale)
		fmt.Printf("[%s] code=%-24s http=%d msg=%s\n",
			locale, errors.CodeOf(notFound), errors.StatusCode(notFound), errors.Resolve(ctx, notFound))
		fmt.Printf("[%s] code=%-24s http=%d msg=%s\n",
			locale, errors.CodeOf(insufficient), errors.StatusCode(insufficient), errors.Resolve(ctx, insufficient))
		fmt.Printf("[%s] code=%-24s http=%d msg=%s\n",
			locale, errors.CodeOf(unknownCode), errors.StatusCode(unknownCode), errors.Resolve(ctx, unknownCode))
		fmt.Println("---")
	}
}
