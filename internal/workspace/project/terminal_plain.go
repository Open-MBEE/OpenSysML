//go:build wasm

package project

import "os"

// isTerminal reports whether f is a terminal. No WebAssembly host offers the query
// (WASI preview 1 has no isatty, a browser has no device), so standard input is
// never reported as one: a "-" that named it reads the lines the host sends and
// ends at end of input, which is what a redirected pipe already does natively.
func isTerminal(*os.File) bool { return false }
