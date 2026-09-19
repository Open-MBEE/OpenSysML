// Package corpus holds the gates over the pinned OMG corpora and the RDF round
// trip of every model under examples/.
package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// One mechanism for the four pinned OMG model roots, two policies: training is
// asserted clean, the other three ratchet per file (docs/project/pilot-corpora.md).

// corpusRoot is one model root of a gate: a repository-relative directory and
// the scope of the diagnostics counted in it.
type corpusRoot struct {
	name       string // label in log lines and, for multi-root gates, expectation paths
	dir        string
	errorsOnly bool // count error diagnostics only, ignoring lower severities
	sysmlOnly  bool // walk .sysml files only, ignoring .kerml
}

// corpusGate is the shared mechanism behind an OMG corpus gate: which roots it
// covers, where its expectations live, and how it announces an absent corpus.
type corpusGate struct {
	name       string
	roots      []corpusRoot
	expected   string
	requireEnv string // set in CI, where an absent corpus fails instead of skipping
	absent     string // "The OMG training corpus is absent, so ..."
	fetch      string // "Fetch it with ./scripts/download-training-examples.sh and re-run"
	runPattern string
	skipHint   string
}

var trainingGate = corpusGate{
	name: "training",
	roots: []corpusRoot{{
		name:       "training",
		dir:        "../../examples/sysml-v2-training",
		errorsOnly: true,
		sysmlOnly:  true,
	}},
	expected:   "testdata/training_examples_expected.txt",
	requireEnv: "OPENSYSML_REQUIRE_TRAINING_CORPUS",
	absent:     "The OMG training corpus is absent, so this run proves nothing about it.",
	fetch:      "Fetch it with ./scripts/download-training-examples.sh and re-run",
	runPattern: "TestTrainingExamples",
	skipHint:   "training examples not downloaded (run ./scripts/download-training-examples.sh)",
}

var pilotCorporaGate = corpusGate{
	name: "pilot-corpora",
	roots: []corpusRoot{
		{name: "kerml-examples", dir: "../../examples/pilot-corpora/kerml-examples"},
		{name: "sysml-examples", dir: "../../examples/pilot-corpora/sysml-examples"},
		{name: "sysml-validation", dir: "../../examples/pilot-corpora/sysml-validation"},
	},
	expected:   "testdata/pilot_corpora_expected.txt",
	requireEnv: "OPENSYSML_REQUIRE_PILOT_CORPORA",
	absent:     "The OMG pilot corpora are absent, so this run proves nothing about them.",
	fetch:      "Fetch them with ./scripts/download-pilot-corpora.sh and re-run",
	runPattern: "TestPilotCorpora",
	skipHint:   "pilot corpora not downloaded (run ./scripts/download-pilot-corpora.sh)",
}

// skip skips locally but fails when the require-env is set, so the gate cannot
// pass by skipping. Announced on stderr because -v-less runs hide skip reasons.
func (g corpusGate) skip(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv(g.requireEnv) != "" {
		t.Fatalf("%s=%s but %s: %s", g.requireEnv, os.Getenv(g.requireEnv), reason, g.skipHint)
	}
	fmt.Fprintf(os.Stderr, "\n!!! GATE NOT RUN: %s SKIPPED - %s.\n"+
		"!!! %s\n"+
		"!!! %s\n"+
		"!!!   go test -count=1 ./tests/corpus -run %s\n"+
		"!!! CI sets %s=1, where an absent corpus fails instead of skipping.\n\n",
		t.Name(), reason, g.absent, g.fetch, g.runPattern, g.requireEnv)
	t.Skip(g.skipHint)
}

// key is the path a diagnostic count is recorded under: a single-root gate needs
// no prefix, a multi-root one is qualified by its root.
func (g corpusGate) key(root corpusRoot, rel string) string {
	if len(g.roots) == 1 {
		return rel
	}
	return root.name + "/" + rel
}

// files returns each root's model files as sorted slash-separated paths relative
// to that root, so the expectations are machine-independent.
func (g corpusGate) files(t *testing.T) map[string][]string {
	t.Helper()

	roots := make(map[string][]string, len(g.roots))
	for _, root := range g.roots {
		if _, err := os.Stat(root.dir); os.IsNotExist(err) {
			g.skip(t, root.dir+" is missing")
		}

		var files []string
		err := filepath.WalkDir(root.dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !root.wants(path) {
				return nil
			}
			rel, err := filepath.Rel(root.dir, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root.dir, err)
		}
		sort.Strings(files)
		// An empty directory is a broken download or a bad cache restore, not a corpus.
		if len(files) == 0 {
			g.skip(t, root.dir+" holds no model files")
		}
		roots[root.name] = files
	}
	return roots
}

// wants reports whether a file belongs to this root's counted set.
func (r corpusRoot) wants(path string) bool {
	switch source.KindOf(path) {
	case source.KindSysML:
		return true
	case source.KindKerML:
		return !r.sysmlOnly
	default:
		return false
	}
}

// counts loads every root and returns the number of diagnostics per recorded
// path, logging each file's messages.
func (g corpusGate) counts(t *testing.T, files map[string][]string) map[string]int {
	t.Helper()

	got := make(map[string]int)
	for _, root := range g.roots {
		for _, batch := range languageBatches(files[root.name]) {
			for key, count := range g.batchCounts(t, root, batch) {
				got[key] = count
			}
		}
	}
	return got
}

// batchCounts loads one language batch of a root and counts its diagnostics. Every
// file is opened before any is diagnosed, because the corpora import across files.
func (g corpusGate) batchCounts(t *testing.T, root corpusRoot, files []string) map[string]int {
	t.Helper()

	got := make(map[string]int, len(files))
	ws := model.NewWorkspace()
	current := ""
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic while loading %s/%s: %v", root.name, current, r)
		}
	}()
	for _, rel := range files {
		current = rel
		content, err := os.ReadFile(filepath.Join(root.dir, rel))
		if err != nil {
			t.Fatalf("read %s/%s: %v", root.name, rel, err)
		}
		ws.Open(rel, content, 1)
	}

	for _, rel := range files {
		current = rel

		var messages []string
		for _, d := range ws.Diagnostics(rel) {
			if root.errorsOnly && d.Severity != diag.SeverityError {
				continue
			}
			messages = append(messages, d.Severity.String()+": "+d.Message)
		}
		if len(messages) > 0 {
			got[g.key(root, rel)] = len(messages)
			t.Logf("%s/%s: %s", root.name, rel, strings.Join(messages, "; "))
		}
	}
	return got
}

// languageBatches splits a root's files into one batch per language, SysML first:
// as in tools/referee/diff, KerML and SysML files must not share a workspace.
func languageBatches(files []string) [][]string {
	var sysml, kerml []string
	for _, rel := range files {
		if source.KindOf(rel) == source.KindKerML {
			kerml = append(kerml, rel)
			continue
		}
		sysml = append(sysml, rel)
	}
	var batches [][]string
	for _, batch := range [][]string{sysml, kerml} {
		if len(batch) > 0 {
			batches = append(batches, batch)
		}
	}
	return batches
}

// readExpected returns the recorded size of each root and the expected
// diagnostic count per recorded path. Lines are "<count>\t<path>"; blank and
// #-prefixed lines are comments, and a root's size is "# files: <n>" for a
// single-root gate and "# files: <root> <n>" otherwise.
func (g corpusGate) readExpected(t *testing.T) (map[string]int, map[string]int) {
	t.Helper()

	content, err := os.ReadFile(g.expected)
	if err != nil {
		t.Fatalf("read %s: %v", g.expected, err)
	}

	totals := make(map[string]int, len(g.roots))
	want := make(map[string]int)
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "# files: "); ok {
			root, count, found := strings.Cut(rest, " ")
			if !found {
				if len(g.roots) > 1 {
					t.Fatalf("%s:%d: want \"# files: <root> <n>\", got %q", g.expected, i+1, line)
				}
				root, count = g.roots[0].name, rest
			}
			totals[root], err = strconv.Atoi(count)
			if err != nil {
				t.Fatalf("%s:%d: bad file count: %v", g.expected, i+1, err)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count, path, found := strings.Cut(line, "\t")
		if !found {
			t.Fatalf("%s:%d: want \"<count>\\t<path>\", got %q", g.expected, i+1, line)
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			t.Fatalf("%s:%d: bad count: %v", g.expected, i+1, err)
		}
		want[path] = n
	}
	return totals, want
}

// checkRootSizes fails when a root holds a different number of files than the
// expectations were recorded against, so a partial download cannot pass.
func (g corpusGate) checkRootSizes(t *testing.T, files map[string][]string, totals map[string]int, regenerate string) {
	t.Helper()
	for _, root := range g.roots {
		if totals[root.name] != len(files[root.name]) {
			t.Errorf("%s holds %d model file(s), expectations were recorded against %d; "+
				"re-download the pinned corpus or regenerate with %s",
				root.name, len(files[root.name]), totals[root.name], regenerate)
		}
	}
}

// writeExpected rewrites the expectation file from the given counts, under the
// given comment header.
func (g corpusGate) writeExpected(t *testing.T, header string, files map[string][]string, got map[string]int) {
	t.Helper()

	var b strings.Builder
	b.WriteString(header)
	for _, root := range g.roots {
		if len(g.roots) == 1 {
			fmt.Fprintf(&b, "# files: %d\n", len(files[root.name]))
			continue
		}
		fmt.Fprintf(&b, "# files: %s %d\n", root.name, len(files[root.name]))
	}
	for _, path := range sortedKeys(got) {
		fmt.Fprintf(&b, "%d\t%s\n", got[path], path)
	}

	if err := os.WriteFile(g.expected, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", g.expected, err)
	}
}

// Restoring the persistent library cache instead of parsing the library must not
// change a diagnostic on any of the four roots: run each gate cold, then warm.
func TestCorpusGatesCacheStateIndependent(t *testing.T) {
	for _, gate := range []corpusGate{trainingGate, pilotCorporaGate} {
		t.Run(gate.name, func(t *testing.T) {
			files := gate.files(t)
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			cold := gate.counts(t, files)
			warm := gate.counts(t, files)

			for _, path := range sortedKeys(cold) {
				if warm[path] != cold[path] {
					t.Errorf("%s: %d diagnostic(s) on an empty cache, %d on a populated one",
						path, cold[path], warm[path])
				}
			}
			for _, path := range sortedKeys(warm) {
				if _, ok := cold[path]; !ok {
					t.Errorf("%s: clean on an empty cache, %d diagnostic(s) on a populated one",
						path, warm[path])
				}
			}
		})
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
