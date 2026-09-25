// Package envvar resolves the toolchain's OPENSYSML_-prefixed environment
// variables, falling back to their legacy SYSML_-prefixed names.
package envvar

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// prefix opens every current variable name; legacy names drop the "OPEN".
const prefix = "OPENSYSML_"

// Legacy returns the SYSML_-prefixed name a variable answered to before the
// prefixes were unified.
func Legacy(name string) string {
	return strings.TrimPrefix(name, "OPEN")
}

// warned records the variables a legacy-name warning was already printed for,
// so a fallback consulted on every lookup warns once per process.
var warned sync.Map

// warnf prints a legacy-name warning; a variable so tests can capture it.
var warnf = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// Lookup returns the value of name (an OPENSYSML_-prefixed variable), falling
// back to its legacy SYSML_-prefixed name when name is unset or empty. When
// both are set and non-empty, the OPENSYSML_ value wins. Using only the legacy
// name warns once per variable per process.
func Lookup(name string) string {
	if !strings.HasPrefix(name, prefix) {
		panic(fmt.Sprintf("envvar: %q is not an %s variable", name, prefix))
	}
	if v := os.Getenv(name); v != "" {
		return v
	}
	legacy := Legacy(name)
	v := os.Getenv(legacy)
	if v != "" {
		if _, done := warned.LoadOrStore(name, struct{}{}); !done {
			warnf("warning: %s is deprecated; set %s instead (the legacy name remains accepted)\n", legacy, name)
		}
	}
	return v
}
