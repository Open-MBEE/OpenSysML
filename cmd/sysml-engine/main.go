// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// sysml-engine serves the SysML execution RPCs — ParseSources, Evaluate,
// Instantiate, ExecuteAction and ExecuteState — over protojson-shaped JSON
// rather than protobuf: over stdio with Content-Length framing and a JSON-RPC
// 2.0 envelope, or, built for js/wasm and run with no -stdio, as the
// synchronous globalThis.sysmlEngine.call(method, paramsJSON) a browser page
// invokes. Keeping protobuf out of the binary is what makes it small enough
// to ship to a browser.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
)

// Build metadata, set by the linker: the names match the -X flags the Makefile
// and the release build pass, so a released binary reports what it was built from.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	useStdio := flag.Bool("stdio", false, "Serve stdio (the default outside the js WebAssembly target)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("sysml-engine %s\n", Version)
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		return
	}

	eng, err := engine.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-engine: %v\n", err)
		os.Exit(1)
	}
	os.Exit(serve(eng, *useStdio))
}

// serveStdio serves one client over stdin/stdout to the end of input; logging
// stays off stdout, where a stray line would corrupt the frames.
func serveStdio(eng *engine.Engine) int {
	if err := eng.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "sysml-engine: stdio session ended in a protocol error: %v\n", err)
		return 1
	}
	return 0
}
