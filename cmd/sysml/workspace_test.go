package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The prompt resolves an import nothing declares against the files under the
// working directory, so the directory the REPL is started in is its workspace.
func TestThePromptLoadsAFileUnderTheWorkingDirectoryForAnUnresolvedImport(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "lib.sysml"), "package Lib { attribute a = 3; }\n")
	cmd := exec.Command(binary)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("private import Lib::*;\na\n%workspace\n")
	res := runCommand(t, cmd)
	for _, want := range []string{"note: loaded lib.sysml, which declares Lib", "= 3", "workspace root: "} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("want %q in the transcript:\n%s", want, res.output())
		}
	}
	if strings.Contains(res.stdout, "unresolved reference") {
		t.Errorf("the import stayed unresolved:\n%s", res.output())
	}
}

func TestThePromptLeavesAnImportUnresolvedWhenNoFileUnderTheWorkingDirectoryDeclaresIt(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "lib.sysml"), "package Lib { attribute a = 3; }\n")
	cmd := exec.Command(binary)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("private import Nope::*;\n")
	res := runCommand(t, cmd)
	if !strings.Contains(res.stdout, "unresolved reference: Nope") || strings.Contains(res.stdout, "note: loaded") {
		t.Errorf("transcript:\n%s", res.output())
	}
}
