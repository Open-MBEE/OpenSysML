package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

func write(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A directory is a valid entry point: `sysml /tmp/proj` loads the model files
// under it rather than failing to read a directory.
func TestLoadFilesAcceptsADirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "uses.sysml"), "package Uses { private import Defs::*; part w : Wheel; }\n")
	write(t, filepath.Join(dir, "defs", "defs.sysml"), "package Defs { part def Wheel; }\n")

	sess := repl.NewSession()
	if status, err := loadFiles(sess, []string{dir}); err != nil || status != exitHolds {
		t.Fatalf("loadFiles(%s) = %d, %v", dir, status, err)
	}
	if got := sess.List(); len(got) != 2 {
		t.Fatalf("want both files in the session, got %v", got)
	}
}

func TestLoadFilesAcceptsAGlob(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "b.sysml"), "package B { }\n")

	sess := repl.NewSession()
	if status, err := loadFiles(sess, []string{filepath.Join(dir, "*.sysml")}); err != nil || status != exitHolds {
		t.Fatalf("loadFiles(glob) = %d, %v", status, err)
	}
	if got := sess.List(); len(got) != 2 {
		t.Fatalf("want 2 declarations, got %v", got)
	}
}

func TestLoadFilesReportsAMissingPath(t *testing.T) {
	sess := repl.NewSession()
	status, err := loadFiles(sess, []string{filepath.Join(t.TempDir(), "nope.sysml")})
	if err == nil {
		t.Fatal("expected an error for a path that does not exist")
	}
	if status != exitUnevaluable {
		t.Errorf("status = %d, want %d for a path that could not be read", status, exitUnevaluable)
	}
}

func TestLoadFilesWithoutPathsDoesNothing(t *testing.T) {
	sess := repl.NewSession()
	if status, err := loadFiles(sess, nil); err != nil || status != exitHolds {
		t.Fatalf("loadFiles(nil) = %d, %v", status, err)
	}
	if got := sess.List(); len(got) != 0 {
		t.Fatalf("want an empty session, got %v", got)
	}
}

// TestCheckDirectoryExitStatus checks that a model named as a directory keeps
// the exit-status contract of a model named by file: every verdict held, one of
// them failed, or no check could be made at all.
func TestCheckDirectoryExitStatus(t *testing.T) {
	binary := buildCLI(t)

	dir := t.TempDir()
	write(t, filepath.Join(dir, "defs.sysml"), "package Defs { part def Wheel; }\n")
	write(t, filepath.Join(dir, "checks.sysml"),
		"package Checks {\n    private import Defs::*;\n    part w : Wheel;\n    constraint Held { 1.0 <= 2.0 }\n    constraint Fails { 3.0 <= 2.0 }\n}\n")

	wantReport(t, checkPaths(t, binary, "-constraint", "Checks::Held", dir), 0, "✓ Constraint Checks::Held passed")
	wantReport(t, checkPaths(t, binary, "-constraint", "Checks::Fails", dir), 1, "✗ Constraint Checks::Fails failed")
	wantReport(t, checkPaths(t, binary, "-constraint", "Checks::nosuch", dir), 2, "unresolved reference: Checks::nosuch")

	// A file of the directory that does not analyse cleanly decides nothing, so
	// the run exits 2 and names the file the error is in.
	write(t, filepath.Join(dir, "broken.sysml"), "package Broken {\n    part probe : Nope::Missing;\n}\n")
	wantReport(t, checkPaths(t, binary, "-constraint", "Checks::Held", dir), 2,
		"broken.sysml:2:18: error: unresolved reference: Nope::Missing", "did not analyse cleanly")
	wantReport(t, checkPaths(t, binary, "-validate", dir), 2, "did not analyse cleanly")

	// A glob over the same files answers the same way.
	wantReport(t, checkPaths(t, binary, "-validate", filepath.Join(dir, "*.sysml")), 2, "did not analyse cleanly")
}

// TestCheckGlobExitStatus checks a glob entry point over a model that holds, and
// that a pattern matching no model file is a misuse rather than a verdict.
func TestCheckGlobExitStatus(t *testing.T) {
	binary := buildCLI(t)

	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package A { constraint Held { 1.0 <= 2.0 } }\n")
	write(t, filepath.Join(dir, "b.sysml"), "package B { part def Wheel; }\n")
	write(t, filepath.Join(dir, "README.md"), "not a model\n")

	wantReport(t, checkPaths(t, binary, "-constraint", "A::Held", filepath.Join(dir, "*")), 0, "✓ Constraint A::Held passed")
	wantReport(t, checkPaths(t, binary, "-validate", filepath.Join(dir, "*.kerml")), 2, "no model files match")
}

// TestLoadRefusesTheTranscriptsName checks that a file named as the session's
// transcript is a read failure like any other: refused before anything loads,
// named in the error, and exiting as a run that decided nothing.
func TestLoadRefusesTheTranscriptsName(t *testing.T) {
	const reserved = "<repl>"
	dir := t.TempDir()
	write(t, filepath.Join(dir, reserved), "package FromFile { part def X; }\n")
	write(t, filepath.Join(dir, "ok.sysml"), "package OK { part def Y; }\n")
	t.Chdir(dir)

	sess := repl.NewSession()
	status, err := loadFiles(sess, []string{"ok.sysml", reserved})
	var named *repl.ReservedNameError
	if !errors.As(err, &named) || named.Name != reserved {
		t.Fatalf("loadFiles error = %v, want a *repl.ReservedNameError naming %s", err, reserved)
	}
	if status != exitUnevaluable {
		t.Errorf("status = %d, want %d", status, exitUnevaluable)
	}
	if got := sess.List(); len(got) != 0 {
		t.Errorf("a refused load must load nothing, got %v", got)
	}

	binary := buildCLI(t)
	wantReport(t, checkPathsIn(t, dir, binary, "-validate", reserved), exitUnevaluable,
		"sysml: cannot load <repl>: the name is reserved")
	wantReport(t, checkPathsIn(t, dir, binary, "-constraint", "OK::Held", "ok.sysml", reserved), exitUnevaluable,
		"cannot load <repl>: the name is reserved")
	wantReport(t, checkPathsIn(t, dir, binary, "-validate", "./"+reserved), exitHolds)
}

// checkPaths runs the binary on paths the caller names, rather than on a model
// written to a file for it as check does.
func checkPaths(t *testing.T, binary string, args ...string) runOutcome {
	t.Helper()
	return checkPathsIn(t, "", binary, args...)
}

// checkPathsIn is checkPaths run from dir, so a relative path is read there.
func checkPathsIn(t *testing.T, dir, binary string, args ...string) runOutcome {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		result.status = exit.ExitCode()
	default:
		t.Fatalf("%v: %v\n%s", args, err, result.output())
	}
	return result
}
