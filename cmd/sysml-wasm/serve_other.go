//go:build !js

package main

import "github.com/Open-MBEE/OpenSysML/internal/frontend/combined"

func serve(server *combined.Server, _ bool) int {
	return serveStdio(server)
}
