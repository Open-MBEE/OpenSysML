package notation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// copyFile copies root/rel into dir/rel, creating the directories.
func copyFile(t *testing.T, root, dir, rel string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, rel, content)
}

func writeFile(t *testing.T, dir, rel string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// citedFiles lists every file the verdict and notation tables cite.
func citedFiles(t *testing.T, root string) []string {
	t.Helper()
	edges, err := ReadNotationTable(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	add := func(cite string) {
		if cite == "" {
			return
		}
		file, _, _ := strings.Cut(cite, ":")
		if strings.Contains(cite, "/testdata/") {
			file = cite
		}
		seen[file] = true
	}
	for _, adj := range table {
		add(adj.Implementation)
		add(adj.Test)
	}
	for _, name := range edges.Names() {
		adj, _ := edges.adjudicate(name)
		add(adj.Implementation)
		add(adj.Test)
	}
	var files []string
	for f := range seen {
		files = append(files, f)
	}
	return files
}

// stageRoot builds a repository root holding what the census reads, with the
// committed excerpt standing in for the rendered specification text.
func stageRoot(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	dir := t.TempDir()
	for _, rel := range append(citedFiles(t, root), baseline.PinPath, notationTablePath, baselinePath, censusDocPath) {
		copyFile(t, root, dir, rel)
	}
	excerpt, err := os.ReadFile(excerptPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, defaultTextPath, excerpt)
	var b Baseline
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &b); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, specPinPath, []byte(b.Source.PilotTag+" "+b.Source.ReleaseCommit+" https://github.com/Systems-Modeling/SysML-v2-Release.git\n"))
	return dir
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errs bytes.Buffer
	code = Main(args, &out, &errs)
	return code, out.String(), errs.String()
}

func mustRead(t *testing.T, dir, rel string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// TestCommittedCensusIsCurrent re-measures the census over the committed
// excerpt and expects the committed baseline and document byte for byte.
func TestCommittedCensusIsCurrent(t *testing.T) {
	dir := stageRoot(t)
	before, doc := mustRead(t, dir, baselinePath), mustRead(t, dir, censusDocPath)
	if code, _, stderr := run(t, "-repo", dir, "-measure"); code != 0 {
		t.Fatalf("measure: %d %s", code, stderr)
	}
	if after := mustRead(t, dir, baselinePath); !bytes.Equal(before, after) {
		t.Errorf("%s is stale; run `go run -C tools ./cmd/notation-census -measure`", baselinePath)
	}
	if after := mustRead(t, dir, censusDocPath); !bytes.Equal(doc, after) {
		t.Errorf("%s has a stale generated block; run `go run -C tools ./cmd/notation-census -measure`", censusDocPath)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rowsPath))); err != nil {
		t.Error(err)
	}
	code, stdout, stderr := run(t, "-repo", dir, "-check", "-require-text")
	if code != 0 || !strings.Contains(stdout, "baseline, census document and evidence agree") {
		t.Fatalf("check: %d %s%s", code, stdout, stderr)
	}
}

func TestCheckCatchesBaselineDrift(t *testing.T) {
	dir := stageRoot(t)
	var b Baseline
	if err := json.Unmarshal(mustRead(t, dir, baselinePath), &b); err != nil {
		t.Fatal(err)
	}
	// A gap recorded drawn: drop its row and move its count.
	var gaps []Gap
	for _, g := range b.NonDrawn {
		if g.Name == "general-compartment" {
			b.Totals.Undrawn--
			b.Totals.Drawn++
			for i := range b.Clauses {
				if b.Clauses[i].Number == g.Clause {
					b.Clauses[i].Undrawn--
					b.Clauses[i].Drawn++
				}
			}
			continue
		}
		gaps = append(gaps, g)
	}
	b.NonDrawn = gaps
	data, err := encodeBaseline(&b)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, baselinePath, data)
	code, _, stderr := run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "general-compartment (8.2.3.2) is undrawn but the baseline records it drawn") {
		t.Fatalf("check: %d %s", code, stderr)
	}
}

func TestCheckCatchesAnInconsistentBaseline(t *testing.T) {
	dir := stageRoot(t)
	content := mustRead(t, dir, baselinePath)
	mutated := bytes.Replace(content, []byte(`"verdict": "undrawn"`), []byte(`"verdict": "drawn"`), 1)
	if bytes.Equal(content, mutated) {
		t.Fatal("nothing to mutate")
	}
	writeFile(t, dir, baselinePath, mutated)
	code, _, stderr := run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "a non-drawn row has verdict \"drawn\"") {
		t.Fatalf("check: %d %s", code, stderr)
	}
}

func TestCheckCatchesAStaleDocument(t *testing.T) {
	dir := stageRoot(t)
	content := mustRead(t, dir, censusDocPath)
	mutated := bytes.Replace(content, []byte("| node | "), []byte("| node | 1"), 1)
	if bytes.Equal(content, mutated) {
		t.Fatal("nothing to mutate")
	}
	writeFile(t, dir, censusDocPath, mutated)
	code, _, stderr := run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "a generated block is stale") {
		t.Fatalf("check: %d %s", code, stderr)
	}
	writeFile(t, dir, censusDocPath, bytes.Replace(content, []byte(blockMarker("gaps", "end")), nil, 1))
	if code, _, stderr := run(t, "-repo", dir, "-check"); code == 0 || !strings.Contains(stderr, "missing or misordered") {
		t.Fatalf("check without an end marker: %d %s", code, stderr)
	}
}

func TestCheckCatchesAChangedText(t *testing.T) {
	dir := stageRoot(t)
	text := mustRead(t, dir, defaultTextPath)
	writeFile(t, dir, defaultTextPath, bytes.Replace(text, []byte("\ncomment-node =\n"), []byte("\ncomment-node =\nremark-node =\n"), 1))
	code, _, stderr := run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "no adjudication for production remark-node") {
		t.Fatalf("a new production: %d %s", code, stderr)
	}
	writeFile(t, dir, defaultTextPath, bytes.Replace(text, []byte("\ncomment-node =\n"), []byte("\n"), 1))
	code, _, stderr = run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "adjudicates comment-node, which §8.2.3 no longer states") {
		t.Fatalf("a dropped production: %d %s", code, stderr)
	}
	writeFile(t, dir, defaultTextPath, bytes.Replace(text, []byte("8.3 Abstract Syntax"), []byte("8.3 Abstract"), 1))
	code, _, stderr = run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "no \"8.3 Abstract Syntax\" heading") {
		t.Fatalf("a text from another rendering: %d %s", code, stderr)
	}
}

func TestCheckHandlesAnAbsentText(t *testing.T) {
	t.Setenv(RequireEnv, "")
	dir := stageRoot(t)
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(defaultTextPath))); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "-repo", dir, "-check")
	if code != 0 || !strings.Contains(stdout, "the measurement comparison is skipped") {
		t.Fatalf("check without the text: %d %s%s", code, stdout, stderr)
	}
	code, _, stderr = run(t, "-repo", dir, "-check", "-require-text")
	if code == 0 || !strings.Contains(stderr, "is absent and required") {
		t.Fatalf("-require-text: %d %s", code, stderr)
	}
	t.Setenv(RequireEnv, "1")
	code, _, stderr = run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "is absent and required") {
		t.Fatalf("%s: %d %s", RequireEnv, code, stderr)
	}
	if code, _, stderr := run(t, "-repo", dir, "-measure"); code == 0 || !strings.Contains(stderr, "no such file") {
		t.Fatalf("measure without the text: %d %s", code, stderr)
	}
}

func TestCheckCatchesAPinChange(t *testing.T) {
	dir := stageRoot(t)
	writeFile(t, dir, specPinPath, []byte("2099-01 0000000 https://example.invalid\n"))
	code, _, stderr := run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "pins \"2099-01 0000000 https://example.invalid\" but") {
		t.Fatalf("check: %d %s", code, stderr)
	}
	content := mustRead(t, dir, baselinePath)
	writeFile(t, dir, baselinePath, bytes.Replace(content, []byte(`"pilotTag": "`), []byte(`"pilotTag": "old-`), 1))
	code, _, stderr = run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "records pilot tag old-") {
		t.Fatalf("a baseline from another pin: %d %s", code, stderr)
	}
}

func TestCheckCatchesABrokenCitation(t *testing.T) {
	dir := stageRoot(t)
	rel := "internal/ir/view/annotation_test.go"
	content := mustRead(t, dir, rel)
	writeFile(t, dir, rel, bytes.Replace(content, []byte("func TestViewFrameCompartments("), []byte("func TestViewFrameCompartmentsRenamed("), 1))
	code, _, stderr := run(t, "-repo", dir, "-check")
	if code == 0 || !strings.Contains(stderr, "declares no function TestViewFrameCompartments") {
		t.Fatalf("check: %d %s", code, stderr)
	}
}

func TestMainRejectsBadArguments(t *testing.T) {
	dir := stageRoot(t)
	if code, _, stderr := run(t, "-repo", dir, "-check", "-measure"); code != 2 || !strings.Contains(stderr, "exclude each other") {
		t.Errorf("-check -measure: %d %s", code, stderr)
	}
	if code, _, stderr := run(t, "-repo", dir, "extra"); code != 2 || !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("extra argument: %d %s", code, stderr)
	}
	if code, _, _ := run(t, "-h"); code != 0 {
		t.Errorf("-h: %d", code)
	}
}

func TestWorkPackagesAreOnlyForGaps(t *testing.T) {
	for _, g := range []struct {
		row  Row
		want string
	}{
		{Row{Production{Name: "comment-node", Clause: "8.2.3.4"}, Adjudication{Verdict: VerdictUndrawn}}, "A"},
		{Row{Production{Name: "root-view", Clause: "8.2.3.26"}, Adjudication{Verdict: VerdictApproximate}}, "A"},
		{Row{Production{Name: "send-action-node", Clause: "8.2.3.17"}, Adjudication{Verdict: VerdictApproximate}}, "B"},
		{Row{Production{Name: "n-ary-connection-dot", Clause: "8.2.3.13"}, Adjudication{Verdict: VerdictUndrawn}}, "C"},
		{Row{Production{Name: "message-node-r", Clause: "8.2.3.16"}, Adjudication{Verdict: VerdictUndrawn}}, "C"},
		{Row{Production{Name: "message", Clause: "8.2.3.16"}, Adjudication{Verdict: VerdictUndrawn}}, "D"},
		{Row{Production{Name: "sq-port", Clause: "8.2.3.9"}, Adjudication{Verdict: VerdictUndrawn}}, "D"},
		{Row{Production{Name: "element-inside-textual-compartment", Clause: "8.2.3.2"}, Adjudication{Verdict: VerdictUndrawn}}, "E"},
		{Row{Production{Name: "result-compartment", Clause: "8.2.3.19"}, Adjudication{Verdict: VerdictUndrawn}}, "F"},
		{Row{Production{Name: "compartment", Clause: "8.2.3.2"}, Adjudication{Verdict: VerdictNotApplicable}}, ""},
		{Row{Production{Name: "part", Clause: "8.2.3.11"}, Adjudication{Verdict: VerdictDrawn}}, ""},
	} {
		if got := workPackage(g.row); got != g.want {
			t.Errorf("%s: package %q, want %q", g.row.Name, got, g.want)
		}
	}
}
