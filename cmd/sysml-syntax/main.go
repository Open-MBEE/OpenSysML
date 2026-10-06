// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// sysml-syntax serves the purely syntactic RPCs — Parse, Format and Tokens —
// over protojson-shaped JSON rather than protobuf: over stdio with
// Content-Length framing and a JSON-RPC 2.0 envelope, or, built for js/wasm
// and run with no -stdio, as the synchronous globalThis.sysmlSyntax.call(
// method, paramsJSON) a browser page invokes. It resolves no names and loads
// no standard library, which is what keeps its WebAssembly build small.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/buildinfo"
)

// Build metadata, set by the linker: the names match the -X flags the Makefile
// and the release build pass, so a released binary reports what it was built from.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

// build is what this binary reports about itself: the linker's stamps, or the
// module version and VCS metadata the toolchain recorded when none were passed.
func build() buildinfo.Info {
	return buildinfo.Resolve(buildinfo.Stamps{Version: Version, Commit: Commit, BuildTime: BuildTime, GoVersion: GoVersion})
}

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	useStdio := flag.Bool("stdio", false, "Serve stdio (the default outside the js WebAssembly target)")
	flag.Parse()

	if *showVersion {
		fmt.Print(build().Report("sysml-syntax"))
		return
	}

	os.Exit(serve(*useStdio))
}

// serveStdio serves one client over stdin/stdout to the end of input; logging
// stays off stdout, where a stray line would corrupt the frames.
func serveStdio() int {
	if err := syntax.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "sysml-syntax: stdio session ended in a protocol error: %v\n", err)
		return 1
	}
	return 0
}
