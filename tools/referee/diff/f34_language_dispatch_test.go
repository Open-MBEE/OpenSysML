package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F34: a root holding both languages must validate each file against the
// reference for its own extension, and neither validator may be handed a file
// of the other language.
func TestRunDispatchesBothLanguagesInOneRoot(t *testing.T) {
	repo := t.TempDir()
	sysmlLog := filepath.Join(repo, "sysml-args.txt")
	kermlLog := filepath.Join(repo, "kerml-args.txt")
	validator, kermlValidator := writeMixedRoot(t, repo, sysmlLog, kermlLog)

	out := filepath.Join(repo, "out")
	if err := run(options{repo: repo, validator: validator, kermlValidator: kermlValidator, out: out}); err != nil {
		t.Fatal(err)
	}

	var report Report
	encoded, err := os.ReadFile(filepath.Join(out, "pilot-diff.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &report); err != nil {
		t.Fatal(err)
	}

	if len(report.Roots) != 1 || report.Roots[0].Totals.Files != 2 {
		t.Fatalf("roots = %+v", report.Roots)
	}
	got := map[string]Entry{}
	for _, file := range report.Roots[0].Files {
		if len(file.PilotOnly) != 1 {
			t.Fatalf("%s pilotOnly = %+v", file.Path, file.PilotOnly)
		}
		got[file.Path] = file.PilotOnly[0]
	}
	if entry := got["Model.sysml"]; entry.Line != 3 || entry.Category != CategorySyntax {
		t.Errorf("Model.sysml pilotOnly = %+v", entry)
	}
	if entry := got["Lib.kerml"]; entry.Line != 2 || entry.Category != CategoryUnresolved {
		t.Errorf("Lib.kerml pilotOnly = %+v", entry)
	}

	sysmlArgs, kermlArgs := readFile(t, sysmlLog), readFile(t, kermlLog)
	if !strings.Contains(sysmlArgs, "Model.sysml") || strings.Contains(sysmlArgs, "Lib.kerml") {
		t.Errorf("the SysML validator was invoked with %q", sysmlArgs)
	}
	if !strings.Contains(kermlArgs, "Lib.kerml") || strings.Contains(kermlArgs, "Model.sysml") {
		t.Errorf("the KerML validator was invoked with %q", kermlArgs)
	}
}

// F34: the KerML reference is now needed by any root holding a .kerml file, so
// its absence must name the script that provisions it rather than dropping the
// file from the comparison.
func TestRunReportsTheMissingKerMLValidator(t *testing.T) {
	repo := t.TempDir()
	validator, _ := writeMixedRoot(t, repo, filepath.Join(repo, "sysml-args.txt"), filepath.Join(repo, "kerml-args.txt"))

	err := run(options{repo: repo, validator: validator,
		kermlValidator: filepath.Join(repo, "absent", "validate-kerml"), out: filepath.Join(repo, "out")})
	if err == nil || !strings.Contains(err.Error(), "download-pilot-kerml-validator.sh") {
		t.Fatalf("run() error = %v", err)
	}
}

// writeMixedRoot lays out a repository with one root holding a file of each
// language, and stub validators that log their arguments and report one
// diagnostic each. It returns the two validator paths.
func writeMixedRoot(t *testing.T, repo, sysmlLog, kermlLog string) (string, string) {
	t.Helper()
	write := func(rel, content string, mode os.FileMode) string {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}

	// The run records its provenance, so a synthetic repository needs the pin
	// and the bridge sources it identifies its inputs by.
	write("scripts/pilot-pin.sh", "PILOT_TAG=\"${PILOT_TAG:-2026-05}\"\nPILOT_COMMIT=\"${PILOT_COMMIT:-fa709f28dfd49dfdb7ee83e4e19da2f57e0eb3aa}\"\nPILOT_ARTIFACT_VERSION=\"${PILOT_ARTIFACT_VERSION:-0.60.1}\"\n", 0o644)
	write("scripts/pilot-sysml-validator/ValidateSysML.java", "class ValidateSysML {}\n", 0o644)
	write("scripts/pilot-kerml-validator/ValidateKerML.java", "class ValidateKerML {}\n", 0o644)

	write("mixed/Model.sysml", "package P;\n", 0o644)
	write("mixed/Lib.kerml", "package Q;\n", 0o644)
	write("bin/pilot-pin.txt", "sysml.release.tag=2026-05\nsysml.artifact.version=0.60.1\n", 0o644)
	validator := write("bin/validate-sysml-batch", stubValidator(sysmlLog,
		"Model.sysml:3:1: error: no viable alternative at input 'x'"), 0o700)
	kermlValidator := write("bin/validate-kerml", stubValidator(kermlLog,
		"Lib.kerml:2:1: error: Couldn't resolve reference to Type 'T'."), 0o700)

	saved := defaultRoots
	defaultRoots = []corpusRoot{{Name: "mixed", Dir: "mixed"}}
	t.Cleanup(func() { defaultRoots = saved })

	return validator, kermlValidator
}

func stubValidator(log, diagnostic string) string {
	return "#!/bin/sh\nprintf '%s\\n' \"$@\" >> " + log + "\necho \"" + diagnostic + "\" >&2\nexit 1\n"
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
