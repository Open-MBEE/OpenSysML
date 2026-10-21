package hygiene

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Caps on a test file and a fixture; the allowlists below record the files that
// predate them, each at its current size, so none of them can grow further.
const (
	maxTestFileLines = 2000
	maxFixtureBytes  = 200 * 1024
)

// oversize is an allowlisted file: its tracked path, its size when recorded (in
// the unit of its check) and the one-line reason it is still over the cap.
type oversize struct {
	path   string
	size   int
	reason string
}

// oversizeTestFiles lists every `_test.go` file over maxTestFileLines, in lines.
// Split one along the feature seams of AGENTS.md §5.2 and remove its entry.
var oversizeTestFiles = []oversize{
	{"tests/export/export_test.go", 4376, "one black-box suite over the whole RDF export surface"},
	{"internal/exec/runtime/classifier_behavior_test.go", 3442, "classifier-behavior scenarios on one shared harness"},
	{"internal/exec/runtime/classify_test.go", 2720, "classification-change scenarios on one shared harness"},
	{"internal/exec/runtime/conformance_test.go", 2748, "execution-conformance driver and its per-case assertions"},
	{"internal/exec/runtime/signal_test.go", 2491, "signal delivery scenarios on one shared harness"},
	{"tests/migrate/behavior_more_test.go", 2465, "SysML v1 behavior migration scenarios on one shared harness"},
	{"internal/frontend/lsp/debug_test.go", 2203, "LSP debug-protocol scenarios over one wire fixture"},
	{"internal/exec/runtime/signal_injection_test.go", 2181, "signal-injection scenarios on one shared harness"},
	{"internal/exec/runtime/adopt_test.go", 2163, "adoption scenarios on one shared harness"},
	{"internal/doc/queryexec/execute_test.go", 2114, "query-execution scenarios on one shared harness"},
	{"internal/exec/runtime/debug_api_test.go", 2094, "debug-API scenarios on one shared harness"},
	{"internal/exec/runtime/replay_test.go", 2025, "replay scenarios on one shared harness"},
}

// oversizeFixtures lists every fixture over maxFixtureBytes, in bytes.
// Shrink the model a golden comes from, or digest it, and remove its entry.
var oversizeFixtures = []oversize{}

// TestTestFilesStayUnderTheLineCap fails when a tracked `_test.go` file is over
// maxTestFileLines without an oversizeTestFiles entry, when an allowlisted file
// has grown past its recorded size, or when its entry has nothing left to excuse.
func TestTestFilesStayUnderTheLineCap(t *testing.T) {
	var files []string
	for _, path := range trackedFiles(t) {
		if strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
	}
	ratchet(t, files, sizeCap{
		kind:      "test file",
		allowlist: oversizeTestFiles,
		limit:     maxTestFileLines,
		unit:      "lines",
		listName:  "oversizeTestFiles",
		remedy:    "split it along the feature seams AGENTS.md §5.2 describes, one file per feature",
		measure:   countLines,
	})
}

// TestFixturesStayUnderTheSizeCap fails when a tracked fixture (a file under a
// `testdata/` directory or named `*.golden.*`) is over maxFixtureBytes without an
// oversizeFixtures entry, when an allowlisted fixture has grown past its recorded
// size, or when its entry has nothing left to excuse.
func TestFixturesStayUnderTheSizeCap(t *testing.T) {
	var files []string
	for _, path := range trackedFiles(t) {
		if isFixture(path) {
			files = append(files, path)
		}
	}
	ratchet(t, files, sizeCap{
		kind:      "fixture",
		allowlist: oversizeFixtures,
		limit:     maxFixtureBytes,
		unit:      "bytes",
		listName:  "oversizeFixtures",
		remedy:    "shrink the model it is generated from, or replace it with a digest of the output",
		measure:   func(data []byte) int { return len(data) },
	})
}

// countLines counts a final line that lacks its newline too.
func countLines(data []byte) int {
	lines := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func isFixture(path string) bool {
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		if component == "testdata" {
			return true
		}
	}
	base := path[strings.LastIndex(path, "/")+1:]
	i := strings.Index(base, ".golden.")
	return i > 0 && i+len(".golden.") < len(base)
}

// sizeCap is one kind of tracked file's cap: the limit in its unit, the named
// allowlist excusing files over it, the remedy a failure suggests, and the measure.
type sizeCap struct {
	kind      string
	allowlist []oversize
	limit     int
	unit      string
	listName  string
	remedy    string
	measure   func([]byte) int
}

// ratchet reports every file over the cap that the allowlist does not cover
// at its current size, and every allowlist entry that no longer excuses anything.
func ratchet(t *testing.T, files []string, c sizeCap) {
	t.Helper()
	recorded := map[string]oversize{}
	for _, entry := range c.allowlist {
		if _, dup := recorded[entry.path]; dup {
			t.Errorf("%s lists %s twice", c.listName, entry.path)
		}
		recorded[entry.path] = entry
	}

	seen := map[string]bool{}
	var problems []string
	for _, path := range files {
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		size := c.measure(data)
		entry, listed := recorded[path]
		seen[path] = true
		switch {
		case !listed && size > c.limit:
			problems = append(problems, fmt.Sprintf("%s is %d %s, over the cap of %d %s: %s; "+
				"if it has to stay whole, add it to %s with its size and the reason",
				path, size, c.unit, c.limit, c.unit, c.remedy, c.listName))
		case listed && size > entry.size:
			problems = append(problems, fmt.Sprintf("%s grew from %d to %d %s: %s rather than growing it further "+
				"(a justified growth updates its %s entry)",
				path, entry.size, size, c.unit, c.remedy, c.listName))
		case listed && size <= c.limit:
			problems = append(problems, fmt.Sprintf("%s is %d %s, within the cap of %d %s: remove its %s entry",
				path, size, c.unit, c.limit, c.unit, c.listName))
		}
	}
	for _, entry := range c.allowlist {
		if !seen[entry.path] {
			problems = append(problems, fmt.Sprintf("%s is not a tracked %s: remove its %s entry", entry.path, c.kind, c.listName))
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

func trackedFiles(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	return strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
}
