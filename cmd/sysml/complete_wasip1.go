//go:build wasip1

package main

import "github.com/Open-MBEE/OpenSysML/internal/frontend/repl"

// exposeCompletion has no host to expose completion to under WASI.
func exposeCompletion(*repl.Session) {}
