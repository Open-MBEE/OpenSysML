// Command snapshot writes the bundled library's snapshot, the derived artifact
// package libs embeds; run through `go generate ./internal/core/libs`.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// snapshotPath is the committed snapshot, relative to the repository root.
const snapshotPath = "internal/core/libs/stdlib.snapshot"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process exit, so tests can drive it in-process.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "file to write the snapshot to (default "+snapshotPath+" under the repository root)")
	check := flags.Bool("check", false, "fail if the file differs from a fresh snapshot instead of writing it")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if repo.NeedsRoot(*out) {
		root, err := repo.Root()
		if err != nil {
			fmt.Fprintln(stderr, "snapshot:", err)
			return 1
		}
		*out = repo.Resolve(root, *out)
		if *out == "" {
			*out = filepath.Join(root, filepath.FromSlash(snapshotPath))
		}
	}

	data, err := libs.BuildSnapshot(libs.BundledSource())
	if err != nil {
		fmt.Fprintln(stderr, "snapshot:", err)
		return 1
	}
	if *check {
		have, err := os.ReadFile(*out)
		if err != nil || !bytes.Equal(have, data) {
			fmt.Fprintf(stderr, "snapshot: %s is stale; run `go generate ./internal/core/libs`\n", *out)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		fmt.Fprintln(stderr, "snapshot:", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", *out, len(data))
	return 0
}
