package diff

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// The reference is handed the OpenSysML libraries beside the standard library,
// once per batch and before the corpus, so a model importing one of them is
// compared on its own diagnostics rather than on an unresolved-reference cascade.
func TestPilotDiagnosticsHandsTheReferenceTheLibraries(t *testing.T) {
	repo := t.TempDir()
	model := filepath.Join(repo, "Model.sysml")
	if err := os.WriteFile(model, []byte("package P;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	libraries := filepath.Join(repo, "OpenSysML Libraries")
	if err := os.Mkdir(libraries, 0o755); err != nil {
		t.Fatal(err)
	}

	log := filepath.Join(repo, "args.txt")
	validator := filepath.Join(repo, "validate-sysml-batch")
	if err := os.WriteFile(validator, []byte(stubValidator(log, "Model.sysml:1:1: warning: w")), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := pilotDiagnostics(validator, libraries, repo, ".", []string{"Model.sysml"}, 0, io.Discard); err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(readFile(t, log)), "\n")
	want := []string{"--extension-library", libraries, "--root", repo, model}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("validator args = %q, want %q", args, want)
	}

	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if _, err := pilotDiagnostics(validator, "", repo, ".", []string{"Model.sysml"}, 0, io.Discard); err != nil {
		t.Fatal(err)
	}
	if args := readFile(t, log); strings.Contains(args, "--extension-library") {
		t.Errorf("no libraries were named, yet the validator was handed some:\n%s", args)
	}
}

// The libraries are part of what a run measures, so the baseline records them as
// an input this repository owns: a changed library is a movement to adjudicate.
func TestProvenanceRecordsTheLibraries(t *testing.T) {
	repo, current := currentProvenance(t)

	var recorded *baseline.Input
	for i := range current.Inputs {
		if current.Inputs[i].Name == librariesInput {
			recorded = &current.Inputs[i]
		}
	}
	if recorded == nil {
		t.Fatalf("provenance records no %q input:\n%+v", librariesInput, current.Inputs)
	}
	if recorded.Dir != defaultLibraries || recorded.Origin != baseline.OriginOurs {
		t.Errorf("libraries input = %+v, want dir %q of origin %q", *recorded, defaultLibraries, baseline.OriginOurs)
	}
	files, err := collectFiles(repo, corpusRoot{Name: librariesInput, Dir: defaultLibraries})
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Files != len(files) || len(files) == 0 {
		t.Errorf("libraries input counts %d files, the directory holds %d", recorded.Files, len(files))
	}

	moved := filepath.Join(t.TempDir(), "libraries")
	if err := os.Mkdir(moved, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := provenance(repo, "", relativeTo(repo, moved)); err == nil {
		t.Error("a library directory holding no model files was accepted")
	}
}

// The libraries handed over are the ones inside the standard-library root this
// implementation loads, so an OPENSYSML_LIBRARY_PATH override moves both sides
// of the comparison together instead of only the reference's.
func TestResolveFollowsTheLibraryRootOpenSysMLLoads(t *testing.T) {
	repo := t.TempDir()
	validator := filepath.Join(repo, "validate-sysml-batch")
	if err := os.WriteFile(validator, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module github.com/Open-MBEE/OpenSysML\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundled := filepath.Join(repo, filepath.FromSlash(defaultLibraries))
	override := filepath.Join(repo, "libraries", openSysMLLibraries)
	for _, dir := range []string{bundled, override} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	clearLibraryOverride(t)
	opts := options{repo: repo, validator: validator, log: io.Discard}
	if err := opts.resolve(); err != nil {
		t.Fatal(err)
	}
	if opts.libraries != bundled {
		t.Errorf("libraries = %s, want the bundled %s", opts.libraries, bundled)
	}

	t.Setenv(libs.LibraryPathEnvVar, filepath.Dir(override))
	opts = options{repo: repo, validator: validator, log: io.Discard}
	if err := opts.resolve(); err != nil {
		t.Fatal(err)
	}
	if opts.libraries != override {
		t.Errorf("libraries = %s, want the overriding %s", opts.libraries, override)
	}
}

// A library directory outside the repository could not be recorded in a
// committed baseline, so resolve refuses an override there rather than
// measuring it silently.
func TestResolveRefusesLibrariesOutsideTheRepository(t *testing.T) {
	repo := t.TempDir()
	validator := filepath.Join(repo, "validate-sysml-batch")
	if err := os.WriteFile(validator, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module github.com/Open-MBEE/OpenSysML\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, openSysMLLibraries), 0o755); err != nil {
		t.Fatal(err)
	}

	clearLibraryOverride(t)
	t.Setenv(libs.LibraryPathEnvVar, outside)
	opts := options{repo: repo, validator: validator, log: io.Discard}
	err := opts.resolve()
	if err == nil || !strings.Contains(err.Error(), "outside the repository") {
		t.Errorf("resolve() = %v, want a refusal naming the repository", err)
	}

	clearLibraryOverride(t)
	opts = options{repo: repo, validator: validator, log: io.Discard}
	err = opts.resolve()
	if err == nil || !strings.Contains(err.Error(), "library directory not found") {
		t.Errorf("resolve() without the bundled libraries = %v, want it to name the missing directory", err)
	}
}

// clearLibraryOverride unsets both spellings of the library-root override for
// the test, so the bundled tree is what resolve derives the libraries from.
func clearLibraryOverride(t *testing.T) {
	t.Setenv(libs.LibraryPathEnvVar, "")
	t.Setenv(envvar.Legacy(libs.LibraryPathEnvVar), "")
}
