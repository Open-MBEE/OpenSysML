package wasm

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRunnerDeadline exercises the harness's own deadline against a runtime that
// answers and then never exits, the way a deadlocked Node exit looks: the run must
// end at the deadline, not the test's, and fail with a report that says how long the
// process ran, what it was, what its threads were doing and what it wrote — so a
// hang on CI is explainable from its log.
func TestRunnerDeadline(t *testing.T) {
	requireNode(t)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("node vanished from PATH: %v", err)
	}
	r := runner{
		target:  wasmTarget{name: "stall", goos: "js"},
		prefix:  []string{node, fixture(t, "stall.mjs")},
		env:     childEnv(t),
		stdin:   stdinPipe,
		timeout: 2 * time.Second,
	}

	t.Run("a process that exits is not a hang", func(t *testing.T) {
		res, err := r.complete(nil, "answer", "-version")
		if err != nil {
			t.Fatalf("complete: %v", err)
		}
		if res.code != 0 || !strings.Contains(res.output, "stall.mjs: started answer -version") {
			t.Errorf("complete = %d, %q; want 0 and the runtime's greeting", res.code, res.output)
		}
	})

	t.Run("a process that never exits fails at the deadline with a report", func(t *testing.T) {
		started := time.Now()
		res, err := r.complete(nil, "stall", "-version")
		took := time.Since(started)
		var hang *hangError
		if !errors.As(err, &hang) {
			t.Fatalf("complete = %d, %v; want a *hangError", res.code, err)
		}
		if took < r.timeout || took > r.timeout+30*time.Second {
			t.Errorf("the run took %s, want about the %s deadline", took, r.timeout)
		}
		report := hang.Error()
		want := []string{
			"no answer within 2s",
			"elapsed: 2.",
			"command: " + strings.Join(r.prefix, " ") + " stall -version",
			"process at the deadline:",
			"output so far:\nstall.mjs: started stall -version",
		}
		if runtime.GOOS == "linux" {
			want = append(want, "State: S (sleeping)", "VmRSS:", "Threads:", `thread `, `"MainThread"`)
		}
		for _, w := range want {
			if !strings.Contains(report, w) {
				t.Errorf("the report is missing %q:\n%s", w, report)
			}
		}
	})
}
