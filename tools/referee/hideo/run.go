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
)

// ErrSuiteAbsent reports that the suite has not been checked out.
var ErrSuiteAbsent = errors.New("the hideo67/sysmlv2-tests suite is not checked out; fetch it with ./scripts/download-hideo-suite.sh")

// Layout of the checkout, relative to its root.
const (
	suiteDir   = "test-suite"
	ResultsDir = suiteDir + "/results"
	pythonDir  = suiteDir + "/test-opensysml-py"
	mdkLink    = "OpenSysML"
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

// Execute builds the service the Python suites drive, provisions their
// environment and runs the four suites, leaving their results under ResultsDir.
func Execute(ctx context.Context, dir, repo string, log io.Writer) error {
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
	for _, suite := range Suites {
		if err := os.Remove(resultsPath(dir, suite)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	run := command(ctx, filepath.Join(dir, suiteDir), log, append([]string{"./run-all.sh"}, Suites...)...)
	// A cached go test would write no results; the suite's Go readers must run every time.
	run.Env = append(os.Environ(), "GOFLAGS=-count=1")
	// Its status reports failing cases and its own cross-MDK report, neither of which
	// is this ratchet's verdict: only a runner that could not start is an error.
	var exit *exec.ExitError
	if err := run.Run(); err != nil && !errors.As(err, &exit) {
		return fmt.Errorf("run-all.sh: %w", err)
	}
	return nil
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
