//go:build wasip1

package main

import "github.com/Open-MBEE/OpenSysML/internal/frontend/repl"

// hostInterruptLine is empty: WASI input is a plain stream with no interrupt.
const hostInterruptLine = ""

// exposeCompletion has no host to expose completion to under WASI.
func exposeCompletion(*repl.Session) {}
