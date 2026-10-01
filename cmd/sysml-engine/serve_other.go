//go:build !js

package main

import "github.com/Open-MBEE/OpenSysML/internal/frontend/engine"

// serve runs the stdio session: native and wasip1 builds have no JS host to
// install a function into, so the pipe protocol is the only surface.
func serve(eng *engine.Engine, _ bool) int {
	return serveStdio(eng)
}
