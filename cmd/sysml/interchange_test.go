package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// interchangeFixture is the committed project tree: a project whose model
// imports Widgets, installed under .sysand/lib and using Gears, and
// VendoredParts, installed under a store only a library root names.
func interchangeFixture(t *testing.T) (proj, store string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "tests", "testdata", "interchange"))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "proj"), filepath.Join(root, "store")
}

// runSysml runs the binary with args and env, returning what it printed and its status.
func runSysml(t *testing.T, binary string, env []string, args ...string) runOutcome {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(cmd.Environ(), env...)
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
		t.Fatalf("running sysml %v: %v\n%s", args, err, result.output())
	}
	return result
}

func TestValidateProjectDirectoryLoadsTheProjectsItUses(t *testing.T) {
	binary := buildCLI(t)
	proj, store := interchangeFixture(t)

	// The project's own library answers Widgets and, through it, Gears; the
	// store nothing names leaves VendoredParts unresolved, and nothing reads
	// the unrelated hidden directory or the older Gears.
	result := runSysml(t, binary, nil, "-validate", proj)
	if result.status == 0 || strings.Count(result.stderr, "unresolved reference: VendoredPart") != 2 || strings.Contains(result.stderr, "Widgets") {
		t.Errorf("-validate %s exited %d:\n%s", proj, result.status, result.output())
	}
	for _, never := range []string{".unrelated", "acme-gears_1.0.0"} {
		if strings.Contains(result.output(), never) {
			t.Errorf("-validate %s read %s:\n%s", proj, never, result.output())
		}
	}

	result = runSysml(t, binary, nil, "-library-root", store, "-validate", proj)
	if result.status != 0 || !strings.Contains(result.stdout, "no errors") {
		t.Errorf("-library-root %s -validate %s exited %d:\n%s", store, proj, result.status, result.output())
	}
	for _, declared := range []string{"package Demo", "package Widgets", "package VendoredParts", "package Gears"} {
		if !strings.Contains(result.stdout, declared) {
			t.Errorf("-validate %s did not report %q:\n%s", proj, declared, result.output())
		}
	}

	result = runSysml(t, binary, []string{"OPENSYSML_LIBRARY_ROOTS=" + store}, "-validate", proj)
	if result.status != 0 || !strings.Contains(result.stdout, "no errors") {
		t.Errorf("OPENSYSML_LIBRARY_ROOTS=%s -validate %s exited %d:\n%s", store, proj, result.status, result.output())
	}
}

func TestValidateFileInProjectFindsItsImportsInTheProjectsUsed(t *testing.T) {
	binary := buildCLI(t)
	proj, store := interchangeFixture(t)

	main := filepath.Join(proj, "model", "main.sysml")
	result := runSysml(t, binary, nil, "-library-root", store, "-validate", main)
	if result.status != 0 || !strings.Contains(result.stdout, "no errors") {
		t.Errorf("-validate %s exited %d:\n%s", main, result.status, result.output())
	}

	// An installed project alone finds what it uses in the library it is
	// installed in: the newer Gears, not the older one beside it.
	widgets := filepath.Join(proj, ".sysand", "lib", "acme-widgets_1.0.0")
	result = runSysml(t, binary, nil, "-validate", widgets)
	if result.status != 0 || !strings.Contains(result.stdout, "no errors") || strings.Contains(result.output(), "acme-gears_1.0.0") {
		t.Errorf("-validate %s exited %d:\n%s", widgets, result.status, result.output())
	}
}
