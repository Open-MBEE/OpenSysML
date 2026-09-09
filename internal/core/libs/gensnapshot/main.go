// Command gensnapshot writes the bundled library's snapshot, the derived
// artifact package libs embeds; run through `go generate ./internal/core/libs`.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process exit, so tests can drive it in-process.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gensnapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "stdlib.snapshot", "file to write the snapshot to")
	check := flags.Bool("check", false, "fail if the file differs from a fresh snapshot instead of writing it")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	data, err := libs.BuildSnapshot(libs.EmbeddedSource())
	if err != nil {
		fmt.Fprintln(stderr, "gensnapshot:", err)
		return 1
	}
	if *check {
		have, err := os.ReadFile(*out)
		if err != nil || !bytes.Equal(have, data) {
			fmt.Fprintf(stderr, "gensnapshot: %s is stale; run `go generate ./internal/core/libs`\n", *out)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		fmt.Fprintln(stderr, "gensnapshot:", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", *out, len(data))
	return 0
}
