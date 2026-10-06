// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/buildinfo"
)

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
		fmt.Print(build().Report("sysml-wasm"))
		return
	}

	server, err := combined.New(build().Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-wasm: %v\n", err)
		os.Exit(1)
	}
	os.Exit(serve(server, *useStdio))
}

func serveStdio(server *combined.Server) int {
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "sysml-wasm: stdio session ended in a protocol error: %v\n", err)
		return 1
	}
	return 0
}
