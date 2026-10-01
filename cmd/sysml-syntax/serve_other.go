//go:build !js

package main

// serve runs the stdio session: native and wasip1 builds have no JS host to
// install a function into, so the pipe protocol is the only surface.
func serve(_ bool) int {
	return serveStdio()
}
