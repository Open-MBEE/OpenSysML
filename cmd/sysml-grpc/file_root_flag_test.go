package main

import (
	"flag"
	"io"
	"testing"
)

// -file-root given an empty directory is refused rather than taken as no confinement.
func TestFileRootFlagNeedsADirectory(t *testing.T) {
	fs := flag.NewFlagSet(grpcCommand, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerFlags(fs)
	if err := fs.Parse([]string{"-file-root", ""}); err == nil {
		t.Error("an empty -file-root parsed")
	}
	fs = flag.NewFlagSet(grpcCommand, flag.ContinueOnError)
	opts := registerFlags(fs)
	if err := fs.Parse([]string{"-file-root", "models"}); err != nil || opts.fileRoot != "models" {
		t.Errorf("-file-root models: fileRoot = %q, %v", opts.fileRoot, err)
	}
}
