package hideo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadResults(t *testing.T) {
	csv := "suite,matrix,id,outcome,detail\n" +
		"opensysml-py,A01,A01-001,pass,\n" +
		"opensysml-py,A01,A01-002,fail,\"expected 2, got 1\nacross lines\"\n" +
		"opensysml-py,B02,B02-001,unsupported,no reader\n"
	got, err := readResults(strings.NewReader(csv), "opensysml-py")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["A01-002"] != (Result{Outcome: Fail, Detail: "expected 2, got 1\nacross lines"}) || got["B02-001"].Outcome != Unsupported {
		t.Errorf("results = %v", got)
	}
}

func TestReadResultsRefusesWhatItCannotCompare(t *testing.T) {
	header := "suite,matrix,id,outcome,detail\n"
	tests := map[string]string{
		"another suite":   header + "opensysml-go,A01,A01-001,pass,\n",
		"unknown outcome": header + "opensysml-py,A01,A01-001,xfail,\n",
		"duplicate case":  header + "opensysml-py,A01,A01-001,pass,\nopensysml-py,A01,A01-001,fail,\n",
		"short row":       header + "opensysml-py,A01,A01-001\n",
		"no case id":      header + "opensysml-py,A01,,pass,\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := readResults(strings.NewReader(content), "opensysml-py"); err == nil {
				t.Error("accepted")
			}
		})
	}
}

func TestReadRunLeavesASuiteWithoutResultsAbsent(t *testing.T) {
	dir := t.TempDir()
	results := filepath.Join(dir, filepath.FromSlash(ResultsDir))
	if err := os.MkdirAll(results, 0o750); err != nil {
		t.Fatal(err)
	}
	row := "suite,matrix,id,outcome,detail\nopensysml-go,A01,A01-001,pass,\n"
	if err := os.WriteFile(filepath.Join(results, "opensysml-go.csv"), []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}
	run, err := ReadRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(run) != 1 || run["opensysml-go"]["A01-001"].Outcome != Pass {
		t.Errorf("run = %v", run)
	}
}

func TestLocate(t *testing.T) {
	repo, dir := t.TempDir(), t.TempDir()
	if err := Locate(dir, repo); !errors.Is(err, ErrSuiteAbsent) {
		t.Errorf("an empty directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, suiteDir), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, suiteDir, "run-all.sh"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Locate(dir, repo); err == nil || errors.Is(err, ErrSuiteAbsent) {
		t.Errorf("no OpenSysML sibling: %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, mdkLink)); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	if err := Locate(dir, repo); err == nil || !strings.Contains(err.Error(), "not this repository") {
		t.Errorf("a sibling that is another tree: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, mdkLink)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo, filepath.Join(dir, mdkLink)); err != nil {
		t.Fatal(err)
	}
	if err := Locate(dir, repo); err != nil {
		t.Error(err)
	}
}

// The readers measure this checkout's build whatever service the caller's environment names.
func TestSuiteEnvPinsTheServiceToThisCheckout(t *testing.T) {
	repo := t.TempDir()
	env := suiteEnv([]string{
		"PATH=/usr/bin",
		"OPENSYSML_SERVICE=localhost:50051",
		"OPENSYSML_BINARY=/opt/other/sysml-grpc",
		"GOFLAGS=-mod=mod",
	}, repo)
	got := strings.Join(env, "\n")
	want := strings.Join([]string{
		"PATH=/usr/bin",
		"OPENSYSML_BINARY=" + filepath.Join(repo, "bin", "sysml-grpc"),
		"GOFLAGS=-mod=mod -count=1",
	}, "\n")
	if got != want {
		t.Errorf("environment:\n%s\nwant:\n%s", got, want)
	}
	if bare := suiteEnv(nil, repo); bare[len(bare)-1] != "GOFLAGS=-count=1" {
		t.Errorf("without a GOFLAGS of the caller: %v", bare)
	}
}

// A reader whose cases fail is run to the end and logged; one that cannot start is an error.
func TestReaderRun(t *testing.T) {
	dir, results := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(results, "logs"), 0o750); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(results, "opensysml-py.csv")
	if err := os.WriteFile(stale, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	failing := reader{suite: "opensysml-py", dir: ".", env: []string{"SAID=traceback"}, args: []string{"sh", "-c", "echo $SAID; exit 1"}}
	if err := failing.run(context.Background(), dir, results, os.Environ(), &log); err != nil {
		t.Fatalf("a reader with failing cases: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the results of an earlier run survive: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(results, "logs", "opensysml-py.log"))
	if err != nil || string(out) != "traceback\n" {
		t.Errorf("the reader's log = %q, %v", out, err)
	}
	if strings.Contains(log.String(), "traceback") || !strings.Contains(log.String(), "== opensysml-py") {
		t.Errorf("the harness log carries the reader's output, or does not name it:\n%s", log.String())
	}

	absent := reader{suite: "opensysml-py", dir: ".", args: []string{filepath.Join(dir, "no-such-interpreter")}}
	if err := absent.run(context.Background(), dir, results, os.Environ(), &log); err == nil || !strings.Contains(err.Error(), "opensysml-py") {
		t.Errorf("a reader that cannot start: %v", err)
	}
}

// Each of the Suites has a reader, and a runner that runs one differently is refused.
func TestCheckRunner(t *testing.T) {
	if len(readers) != len(Suites) {
		t.Fatalf("%d readers for %d suites", len(readers), len(Suites))
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, suiteDir), 0o750); err != nil {
		t.Fatal(err)
	}
	var script strings.Builder
	for i, r := range readers {
		if r.suite != Suites[i] {
			t.Errorf("reader %d is %s, suite %d is %s", i, r.suite, i, Suites[i])
		}
		script.WriteString(r.suite + ") cd x && " + r.line() + " ;;\n")
	}
	runner := filepath.Join(dir, suiteDir, "run-all.sh")
	if err := os.WriteFile(runner, []byte(script.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkRunner(dir); err != nil {
		t.Errorf("the runner this package follows: %v", err)
	}
	moved := strings.Replace(script.String(), "test_props.py", "test_properties.py", 1)
	if err := os.WriteFile(runner, []byte(moved), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkRunner(dir); err == nil || !strings.Contains(err.Error(), "opensysml-py-props") {
		t.Errorf("a runner that runs a reader differently: %v", err)
	}
}
