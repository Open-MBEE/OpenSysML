//go:build !js

package main

import "github.com/Open-MBEE/OpenSysML/internal/frontend/core"

func serve(c *core.Core, _ bool) int {
	return serveStdio(c)
}
