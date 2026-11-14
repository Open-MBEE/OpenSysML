package hideo

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrSuiteAbsent reports that the suite has not been checked out.
var ErrSuiteAbsent = errors.New("the hideo67/sysmlv2-tests suite is not checked out; fetch it with ./scripts/download-hideo-suite.sh")

// Layout of the checkout, relative to its root.
const (
	suiteDir   = "test-suite"
	ResultsDir = suiteDir + "/results"
	pythonDir  = suiteDir + "/test-opensysml-py"
	goDir      = suiteDir + "/test-opensysml-go"
	mdkLink    = "OpenSysML"
)

// The service the Python readers drive: the build of this repository, and the two
// variables of the client that would have them drive another one.
const (
	serviceBinary = "bin/sysml-grpc"
	binaryEnv     = "OPENSYSML_BINARY"
	serviceEnv    = "OPENSYSML_SERVICE"
)

// Locate checks that dir holds the suite and that its OpenSysML sibling is the
// repository at repo, so the suite builds and reads the tree being measured.
func Locate(dir, repo string) error {
	if _, err := os.Stat(filepath.Join(dir, suiteDir, "run-all.sh")); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w (looked in %s)", ErrSuiteAbsent, dir)
	} else if err != nil {
		return err
	}
	link := filepath.Join(dir, mdkLink)
	linked, err := filepath.EvalSymlinks(link)
	if err != nil {
		return fmt.Errorf("%s must link to this repository: %w; re-run ./scripts/download-hideo-suite.sh", link, err)
	}
	root, err := filepath.EvalSymlinks(repo)
	if err != nil {
		return err
	}
	if linked != root {
		return fmt.Errorf("%s is %s, not this repository (%s); re-run ./scripts/download-hideo-suite.sh from this checkout", link, linked, root)
	}
	return nil
}

// reader is how the suite's own runner, run-all.sh, runs one of the Suites.
type reader struct {
	suite, dir string
	env, args  []string
}

var readers = []reader{
	{"opensysml-py-props", pythonDir, []string{"OPENSYSML_PY_SUITE=opensysml-py-props"}, []string{".venv/bin/python", "-m", "pytest", "-q", "test_props.py"}},
	{"opensysml-go-props", goDir, []string{"OPENSYSML_GO_SUITE=opensysml-go-props"}, []string{"go", "test", "-v", "./..."}},
	{"opensysml-py", pythonDir, nil, []string{".venv/bin/python", "-m", "pytest", "-q", "test_matrices.py"}},
	{"opensysml-go", goDir, nil, []string{"go", "test", "-v", "./..."}},
}

// line is the reader's command as run-all.sh spells it.
func (r reader) line() string {
	return strings.Join(append(append([]string{}, r.env...), r.args...), " ")
}

// checkRunner refuses a pinned runner that no longer runs a reader the way this package does.
func checkRunner(dir string) error {
	path := filepath.Join(dir, suiteDir, "run-all.sh")
	script, err := os.ReadFile(path) // #nosec G304 -- the runner of the suite checkout the caller named
	if err != nil {
		return err
	}
	for _, r := range readers {
		if !strings.Contains(string(script), r.line()) {
			return fmt.Errorf("%s no longer runs %s as `%s`: bring tools/referee/hideo/run.go in line with the pin", path, r.suite, r.line())
		}
	}
	return nil
}

// Execute builds the service the Python suites drive, provisions their environment and
// runs the four readers, leaving their results under ResultsDir and their output under logs/.
func Execute(ctx context.Context, dir, repo string, log io.Writer) error {
	if err := checkRunner(dir); err != nil {
		return err
	}
	steps := []struct {
		dir  string
		args []string
	}{
		{repo, []string{"make", "build-grpc"}},
		{filepath.Join(dir, pythonDir), []string{"uv", "sync"}},
	}
	for _, step := range steps {
		if err := command(ctx, step.dir, log, step.args...).Run(); err != nil {
			return fmt.Errorf("%v in %s: %w", step.args, step.dir, err)
		}
	}
	results, err := filepath.Abs(filepath.Join(dir, filepath.FromSlash(ResultsDir)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(results, "logs"), 0o750); err != nil {
		return err
	}
	env := append(suiteEnv(os.Environ(), repo), "RESULTS_DIR="+results)
	for _, r := range readers {
		if err := r.run(ctx, dir, results, env, log); err != nil {
			return err
		}
	}
	return nil
}

// run runs one reader in place of run-all.sh, whose version and report stages are the
// suite's cross-MDK report and need the pilot beside it. Failing cases are not an error.
func (r reader) run(ctx context.Context, dir, results string, env []string, log io.Writer) error {
	if err := os.Remove(filepath.Join(results, r.suite+".csv")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	path := filepath.Join(results, "logs", r.suite+".log")
	out, err := os.Create(path) // #nosec G304 -- a log of the suite checkout the caller named
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "== %s\n", r.suite)
	cmd := command(ctx, filepath.Join(dir, filepath.FromSlash(r.dir)), out, r.args...)
	cmd.Env = append(append([]string{}, env...), r.env...)
	err = cmd.Run()
	if closeErr := out.Close(); closeErr != nil {
		return closeErr
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		fmt.Fprintf(log, "   %v, as when a case fails; its output is in %s\n", err, path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %v in %s: %w", r.suite, r.args, cmd.Dir, err)
	}
	return nil
}

// suiteEnv is the readers' environment: the service is this checkout's build, never one the
// caller's environment names, and go test is uncached, since a cached run writes no results.
func suiteEnv(environ []string, repo string) []string {
	goflags := "-count=1"
	env := make([]string, 0, len(environ)+2)
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		switch name {
		case serviceEnv, binaryEnv:
		case "GOFLAGS":
			goflags = strings.TrimSpace(value + " " + goflags)
		default:
			env = append(env, entry)
		}
	}
	return append(env, binaryEnv+"="+filepath.Join(repo, filepath.FromSlash(serviceBinary)), "GOFLAGS="+goflags)
}

func command(ctx context.Context, dir string, log io.Writer, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- fixed commands of the suite's documented procedure
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, log, log
	return cmd
}

func resultsPath(dir, suite string) string {
	return filepath.Join(dir, filepath.FromSlash(ResultsDir), suite+".csv")
}

// ReadRun reads the results the suites left under dir; a suite with no results
// file is absent from the run rather than an error, for Compare to report.
func ReadRun(dir string) (Run, error) {
	run := make(Run, len(Suites))
	for _, suite := range Suites {
		path := resultsPath(dir, suite)
		f, err := os.Open(path) // #nosec G304 -- a results file of the suite checkout the caller named
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		results, err := readResults(f, suite)
		closeErr := f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		run[suite] = results
	}
	return run, nil
}

// readResults decodes one suite's CSV: a header, then suite, matrix, id, outcome, detail.
func readResults(r io.Reader, suite string) (map[string]Result, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	results := make(map[string]Result, len(rows))
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) != 5 || row[0] != suite || row[2] == "" {
			return nil, fmt.Errorf("row %d: expected suite %s, a matrix, a case id, an outcome and a detail, got %q", i+1, suite, row)
		}
		outcome, err := parseOutcome(row[3])
		if err != nil {
			return nil, fmt.Errorf("row %d (%s): %w", i+1, row[2], err)
		}
		if _, dup := results[row[2]]; dup {
			return nil, fmt.Errorf("row %d: case %s recorded twice", i+1, row[2])
		}
		results[row[2]] = Result{Outcome: outcome, Detail: row[4]}
	}
	return results, nil
}
