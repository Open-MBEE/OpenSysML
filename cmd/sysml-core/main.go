// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// sysml-core serves parsing, validation diagnostics and symbol facts over
// protojson-shaped JSON without protobuf, Connect or execution dependencies.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/core"
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
		fmt.Print(build().Report("sysml-core"))
		return
	}

	c, err := core.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-core: %v\n", err)
		os.Exit(1)
	}
	os.Exit(serve(c, *useStdio))
}

func serveStdio(c *core.Core) int {
	if err := c.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "sysml-core: stdio session ended in a protocol error: %v\n", err)
		return 1
	}
	return 0
}
