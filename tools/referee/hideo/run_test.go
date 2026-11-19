package hideo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
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

// The Go readers resolve a requirement this module raised past the suite's, through a copy
// of the suite's module files: the suite's own stay as pinned.
func TestGoReadersResolveARaisedRequirementInACopy(t *testing.T) {
	root := t.TempDir()
	mdk, module, scratch := filepath.Join(root, "mdk"), filepath.Join(root, "reader"), filepath.Join(root, "scratch")
	files := map[string]string{
		filepath.Join(root, "dep1", "go.mod"): "module example.test/dep\n\ngo 1.26\n",
		filepath.Join(root, "dep1", "dep.go"): "package dep\n\nconst Version = 1\n",
		filepath.Join(root, "dep2", "go.mod"): "module example.test/dep\n\ngo 1.26\n",
		filepath.Join(root, "dep2", "dep.go"): "package dep\n\nconst Version = 2\n",
		filepath.Join(mdk, "go.mod"):          "module example.test/mdk\n\ngo 1.26\n\nrequire example.test/dep v1.1.0\n",
		filepath.Join(mdk, "mdk.go"):          "package mdk\n\nimport \"example.test/dep\"\n\nconst Version = dep.Version\n",
		filepath.Join(module, "go.mod"): "module example.test/reader\n\ngo 1.26\n\nrequire example.test/mdk v0.0.0\n\n" +
			"require example.test/dep v1.0.0 // indirect\n\nreplace example.test/mdk => ../mdk\n\n" +
			"replace example.test/dep v1.0.0 => ../dep1\n\nreplace example.test/dep v1.1.0 => ../dep2\n",
		filepath.Join(module, "go.sum"): "",
		filepath.Join(module, "reader_test.go"): "package reader\n\nimport (\n\t\"testing\"\n\n\t\"example.test/mdk\"\n)\n\n" +
			"func TestVersion(t *testing.T) {\n\tif mdk.Version != 2 {\n\t\tt.Fatal(mdk.Version)\n\t}\n}\n",
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(scratch, 0o750); err != nil {
		t.Fatal(err)
	}
	modfile, err := scratchModule(module, scratch)
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GOFLAGS=-count=1", "GOWORK=off", "GOPROXY=off")
	run := func(args []string) (string, error) {
		cmd := exec.Command(args[0], args[1:]...) // #nosec G204 -- the go command over a module this test wrote
		cmd.Dir, cmd.Env = module, env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	args := []string{"go", "test", "-v", "./..."}
	if out, err := run(args); err == nil || !strings.Contains(out, "updates to go.mod needed") {
		t.Fatalf("the suite's own module file resolves the raised requirement: %v\n%s", err, out)
	}
	if out, err := run(withModfile(args, modfile)); err != nil {
		t.Fatalf("%v: %v\n%s", withModfile(args, modfile), err, out)
	}
	if got, err := os.ReadFile(filepath.Join(module, "go.mod")); err != nil || string(got) != files[filepath.Join(module, "go.mod")] {
		t.Errorf("the suite's go.mod was rewritten (%v):\n%s", err, got)
	}
	if got, err := os.ReadFile(modfile); err != nil || !strings.Contains(string(got), "example.test/dep v1.1.0") {
		t.Errorf("the copy does not carry the raised requirement (%v):\n%s", err, got)
	}
}

// The readers' arms of run-all.sh as the pin spells them, with a neighbour's.
const pinnedRunner = `run_suite() {
  case "$1" in
    opensysml-py)     cd test-opensysml-py && .venv/bin/python -m pytest -q test_matrices.py ;;
    opensysml-py-props)
                      cd test-opensysml-py &&
                      OPENSYSML_PY_SUITE=opensysml-py-props .venv/bin/python -m pytest -q test_props.py ;;
    opensysml-go)     cd test-opensysml-go && go test -v ./... ;;
    opensysml-go-props)
                      cd test-opensysml-go && OPENSYSML_GO_SUITE=opensysml-go-props go test -v ./... ;;
    opensysml-rs)     cd test-opensysml-rs && cargo test --release -- --nocapture ;;
  esac
}
`

// Each of the Suites has a reader, and a runner that runs one differently is refused.
func TestCheckRunner(t *testing.T) {
	if len(readers) != len(Suites) {
		t.Fatalf("%d readers for %d suites", len(readers), len(Suites))
	}
	for i, r := range readers {
		if r.suite != Suites[i] {
			t.Errorf("reader %d is %s, suite %d is %s", i, r.suite, i, Suites[i])
		}
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, suiteDir), 0o750); err != nil {
		t.Fatal(err)
	}
	check := func(script string) error {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, suiteDir, "run-all.sh"), []byte(script), 0o600); err != nil {
			t.Fatal(err)
		}
		return checkRunner(dir)
	}
	if err := check(pinnedRunner); err != nil {
		t.Errorf("the runner of the pin: %v", err)
	}
	const goArm = "    opensysml-go)     cd test-opensysml-go && go test -v ./... ;;\n"
	for _, tc := range []struct{ name, old, new, suite string }{
		{"another test file", "test_props.py", "test_properties.py", "runs opensysml-py-props as"},
		{"arguments after the command", "OPENSYSML_GO_SUITE=opensysml-go-props go test -v ./...", "OPENSYSML_GO_SUITE=opensysml-go-props go test -v ./... -run TestOther", "runs opensysml-go-props as"},
		{"a reader's arm dropped, its command still within another's", goArm, "", "runs opensysml-go as"},
		{"another directory", "opensysml-py)     cd test-opensysml-py", "opensysml-py)     cd test-other-py", "runs opensysml-py as"},
		{"an environment variable before the command", "&& go test -v ./... ;;", "&& GOFLAGS=-short go test -v ./... ;;", "runs opensysml-go as"},
	} {
		if !strings.Contains(pinnedRunner, tc.old) {
			t.Fatalf("%s: the pinned runner has no %q", tc.name, tc.old)
		}
		err := check(strings.Replace(pinnedRunner, tc.old, tc.new, 1))
		if err == nil || !strings.Contains(err.Error(), tc.suite) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
