package hygiene

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gateTimingRun runs the opt-in timing script with a controlled HOME, so a
// hostile value cannot direct writes at the real cache.
func gateTimingRun(t *testing.T, home, value string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "scripts/gate-timing.sh")
	cmd.Dir = repoRoot
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HOME=") && !strings.HasPrefix(entry, "GATE_TIMING_TMP=") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "HOME="+home, "GATE_TIMING_TMP="+value)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestGateTimingRejectsHostileTmp(t *testing.T) {
	for _, tc := range []struct {
		name, value, message string
	}{
		{"absolute", "/tmp/gate-timing-hostile", "must be relative"},
		{"traversal", "../escape", "must not contain '..'"},
		{"nested traversal", "a/../../escape", "must not contain '..'"},
		{"unsupported character", "a b", "unsupported component"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			out, err := gateTimingRun(t, home, tc.value)
			if err == nil {
				t.Fatalf("accepted GATE_TIMING_TMP=%q", tc.value)
			}
			if !strings.Contains(out, tc.message) {
				t.Errorf("output %q lacks %q", out, tc.message)
			}
			root := filepath.Join(home, ".cache", "opensysml")
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("created %s for a hostile value: %v", root, err)
			}
		})
	}
}

func TestGateTimingRejectsSymlinkedTmp(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".cache", "opensysml")
	outside := filepath.Join(home, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	out, err := gateTimingRun(t, home, "link")
	if err == nil {
		t.Fatal("accepted a symlinked GATE_TIMING_TMP")
	}
	if !strings.Contains(out, "symlinked path component") {
		t.Errorf("output %q does not name the symlink", out)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Errorf("wrote through the symlink into %s: %v, %v", outside, entries, err)
	}
}

// TestGateTimingForcesUncachedRuns pins the methodology: both timed arms run
// after GOFLAGS gains -count=1, so neither can replay a result the other cached.
func TestGateTimingForcesUncachedRuns(t *testing.T) {
	source := readRepoFile(t, "scripts/gate-timing.sh")
	flags := strings.Index(source, "export GOFLAGS=")
	serial := strings.Index(source, "make test-quick SHARDS=")
	runner := strings.Index(source, "go run -C tools ./cmd/gate-runner")
	if flags < 0 || serial < 0 || runner < 0 {
		t.Fatalf("timing script no longer has the GOFLAGS export and both timed arms:\n%s", source)
	}
	if flags > serial || flags > runner {
		t.Errorf("GOFLAGS is exported after a timed arm (flags=%d serial=%d runner=%d)", flags, serial, runner)
	}
	for _, want := range []string{`export GOFLAGS="$GOFLAGS -count=1"`, `export GOFLAGS='-count=1'`} {
		if !strings.Contains(source, want) {
			t.Errorf("timing script lacks %q", want)
		}
	}
	doc := readRepoFile(t, "docs/project/test-gate-throughput.md")
	for _, want := range []string{"GOFLAGS=-count=1", "not cache hits"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the timing documentation no longer says %q", want)
		}
	}
}
