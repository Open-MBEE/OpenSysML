// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
)

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
		fmt.Printf("sysml-wasm %s\n", Version)
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		return
	}

	server, err := combined.New(Version)
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
