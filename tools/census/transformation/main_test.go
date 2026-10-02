package transformation

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// TestCensusIsCurrent is the gate in test form: the committed baseline, the
// census document, the citations and the recorded measurement must agree, and
// the baseline must list what the pinned model contains whenever the XMI is
// provisioned.
func TestCensusIsCurrent(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCheck(root, options{}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

// TestExtractedMappingsMatchBaseline compares a fresh extraction with the
// committed baseline when the model is provisioned, so a stale baseline fails
// here too. Absent, it skips — or fails under OPENSYSML_REQUIRE_SYSML_V1TOV2,
// as CI sets it.
func TestExtractedMappingsMatchBaseline(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := ReadPin(root)
	if err != nil {
		t.Fatal(err)
	}
	xmi := modelPath(root, pin.File, "")
	if _, err := os.Stat(xmi); err != nil {
		if os.Getenv(RequireEnv) == "1" {
			t.Fatalf("the pinned model is required (%s=1): run ./scripts/download-sysml-v1tov2.sh", RequireEnv)
		}
		t.Skipf("pinned model not provisioned at %s (run ./scripts/download-sysml-v1tov2.sh)", xmi)
	}
	if err := verifyPin(pin, xmi); err != nil {
		t.Fatal(err)
	}
	fresh, err := extract(xmi)
	if err != nil {
		t.Fatal(err)
	}
	base, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := base.matchesExtracted(fresh.Mappings); err != nil {
		t.Fatal(err)
	}
}

// TestCheckRejectsAnAbsentRequiredModel: -check -require-xmi turns "no model"
// into a failure rather than a skip. Vacuous while the model is provisioned.
func TestCheckRejectsAnAbsentRequiredModel(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := ReadPin(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(modelPath(root, pin.File, "")); err == nil {
		t.Skip("the pinned model is provisioned; the absent path is exercised without it")
	}
	if err := runCheck(root, options{requireXMI: true}, &bytes.Buffer{}); err == nil {
		t.Fatal("-check -require-xmi over an absent model must fail")
	}
}

// testRoot builds a minimal repository: the pin script, a baseline and a
// census document consistent with it, for the mutation tests below.
func testRoot(t *testing.T, base *Baseline) string {
	t.Helper()
	checkout, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	pin, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(PinPath)))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, filepath.FromSlash(PinPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, pin, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeBaseline(root, base); err != nil {
		t.Fatal(err)
	}
	doc := "# Census\n\n" +
		"<!-- census:begin source -->\nx\n<!-- census:end source -->\n\n" +
		"<!-- census:begin summary -->\nx\n<!-- census:end summary -->\n\n" +
		"<!-- census:begin rows -->\nx\n<!-- census:end rows -->\n\n" +
		"<!-- census:begin beyond -->\nx\n<!-- census:end beyond -->\n\n" +
		"<!-- census:begin errata -->\nx\n<!-- census:end errata -->\n\n" +
		"<!-- census:begin gaps -->\nx\n<!-- census:end gaps -->\n"
	rewritten, err := rewriteBlocks(doc, base)
	if err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docPath, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func testBaseline() *Baseline {
	pin, err := ReadPin(mustRoot())
	if err != nil {
		panic(err)
	}
	return &Baseline{
		Source: Source{
			Document: pin.Document, Version: pin.Version, URL: pin.URL,
			Digest: "sha256:" + pin.SHA256, File: pin.File,
			Packages: 2, Classes: 2, Mappings: 2, OCLBodies: 0,
		},
		Measurement: Measurement{Corpora: []Corpus{}, Counts: map[string]TokenCount{"uml:Class": {}}},
		Mappings: []Mapping{
			{Name: "Alpha_Mapping", Package: "Pkg", Qualified: "Pkg::Alpha_Mapping", From: "Element", To: "Usage",
				Status: StatusUnknown, Reason: "not yet adjudicated", Implementation: []string{}, Tests: []string{}, Scope: []string{}},
			{Name: "Beta_Mapping", Package: "Pkg", Qualified: "Pkg::Beta_Mapping", From: "Class", To: "PartUsage",
				Status: StatusNotImplemented, Reason: "no v2 writer", Implementation: []string{}, Tests: []string{}, Scope: []string{"uml:Class"}},
		},
	}
}

func mustRoot() string {
	root, err := repo.Root()
	if err != nil {
		panic(err)
	}
	return root
}

// runCheckFails asserts the mutation leaves the gate non-zero and names why.
func runCheckFails(t *testing.T, root string, want string) {
	t.Helper()
	err := runCheck(root, options{}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("-check passed; want a failure naming %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want it to name %q", err, want)
	}
}

// TestCheckCatchesAMissingRow: a baseline mapping the document does not name.
func TestCheckCatchesAMissingRow(t *testing.T) {
	base := testBaseline()
	root := testRoot(t, base)
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(content), "| `Beta_Mapping` |", "| REMOVED |", 1)
	if stale == string(content) {
		t.Fatal("the generated row was not found to remove")
	}
	if err := os.WriteFile(docPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	runCheckFails(t, root, "Beta_Mapping is in "+baselinePath+" but has no census row")
}

// TestCheckCatchesAnExtraRow: a census row naming a mapping the baseline lacks.
func TestCheckCatchesAnExtraRow(t *testing.T) {
	base := testBaseline()
	root := testRoot(t, base)
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	anchor := "| `Beta_Mapping` |"
	i := strings.Index(string(content), anchor)
	if i < 0 {
		t.Fatal("the generated row was not found")
	}
	lineEnd := strings.Index(string(content)[i:], "\n")
	row := string(content)[i : i+lineEnd]
	extra := strings.Replace(row, "Beta_Mapping", "Ghost_Mapping", 1)
	mutated := string(content)[:i+lineEnd] + "\n" + extra + string(content)[i+lineEnd:]
	if err := os.WriteFile(docPath, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	runCheckFails(t, root, "Ghost_Mapping is in the census table but not in "+baselinePath)
}

// TestCheckCatchesABadCite: an implementation cite must resolve to a declared
// function of a file that exists.
func TestCheckCatchesABadCite(t *testing.T) {
	base := testBaseline()
	base.Mappings[1].Implementation = []string{"internal/no/such/file.go:NoSuchFunc"}
	base.Mappings[1].Tests = []string{"internal/no/such/file_test.go:TestNoSuch"}
	// An adjudicated status change alone does not trip the document blocks:
	// cites live in the baseline, not the table.
	root := testRoot(t, base)
	runCheckFails(t, root, "internal/no/such/file.go does not exist")
}

// TestCheckCatchesAStaleSummary: a hand-edited census figure is a stale block.
func TestCheckCatchesAStaleSummary(t *testing.T) {
	base := testBaseline()
	root := testRoot(t, base)
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(content), "1 ❔ unknown", "2 ❔ unknown", 1)
	if stale == string(content) {
		t.Fatal("the generated summary was not found to edit")
	}
	if err := os.WriteFile(docPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	runCheckFails(t, root, "a generated block is stale")
}

// TestCheckCatchesExtractionDrift: a baseline row whose recorded `from`
// differs from a fresh extraction must be reported.
func TestCheckCatchesExtractionDrift(t *testing.T) {
	base := testBaseline()
	fresh := make([]Mapping, len(base.Mappings))
	copy(fresh, base.Mappings)
	fresh[1].From = "NamedElement"
	err := base.matchesExtracted(fresh)
	if err == nil || !strings.Contains(err.Error(), "Beta_Mapping extracted fields drifted") {
		t.Fatalf("error = %v, want the drifted field named", err)
	}
	fresh[1].From = "Class"
	if err := base.matchesExtracted(fresh); err != nil {
		t.Fatalf("a matching extraction must compare: %v", err)
	}
}

// TestValidateEnforcesTheAdjudicationRules: the per-row rules the gate applies
// before the document or the model is read.
func TestValidateEnforcesTheAdjudicationRules(t *testing.T) {
	for name, mutate := range map[string]func(*Mapping){
		"unadjudicated without a reason": func(m *Mapping) { m.Reason = "" },
		"unscoped gap":                   func(m *Mapping) { m.Scope = nil },
		"bad token":                      func(m *Mapping) { m.Scope = []string{"kerml:Class"} },
		"unmeasured token":               func(m *Mapping) { m.Scope = []string{"uml:Block"} },
		"backed but uncited":             func(m *Mapping) { m.Status, m.Scope = StatusFaithful, nil },
		"bad status":                     func(m *Mapping) { m.Status = "half-done" },
	} {
		t.Run(name, func(t *testing.T) {
			base := testBaseline()
			mutate(&base.Mappings[1])
			if name == "backed but uncited" {
				base.Measurement.Counts = map[string]TokenCount{}
			}
			if err := base.validate(); err == nil {
				t.Fatalf("row %+v must not validate", base.Mappings[1])
			}
		})
	}
	base := testBaseline()
	if err := base.validate(); err != nil {
		t.Fatalf("the unmodified baseline must validate: %v", err)
	}
}

// TestCheckCatchesABeyondCiteFailure: a beyond entry's citations resolve
// against the tree like a row's.
func TestCheckCatchesABeyondCiteFailure(t *testing.T) {
	base := testBaseline()
	base.Beyond = []BeyondEntry{
		{Behaviour: "a behaviour with no OMG mapping", Implementation: []string{"internal/no/such/file.go:NoSuchFunc"}, Tests: []string{"internal/no/such/file_test.go:TestNoSuch"}},
	}
	root := testRoot(t, base)
	runCheckFails(t, root, "internal/no/such/file.go does not exist")
}

// TestCheckCatchesAStaleErrataBlock: the errata table is generated from the
// reasons that carry the candidate-erratum marker, so a hand edit is stale.
func TestCheckCatchesAStaleErrataBlock(t *testing.T) {
	base := testBaseline()
	base.Mappings[1].Reason = "candidate erratum: writes a Flow, the specification expects a FlowUsage"
	root := testRoot(t, base)
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(content), "the specification expects a FlowUsage", "edited by hand", 1)
	if stale == string(content) {
		t.Fatal("the generated errata note was not found to edit")
	}
	if err := os.WriteFile(docPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	runCheckFails(t, root, "a generated block is stale")
}

// TestGapsGroupByVerdictScopeReason: rows sharing status, scope and reason
// collapse into one ranked entry, and editing its figures is stale.
func TestGapsGroupByVerdictScopeReason(t *testing.T) {
	base := testBaseline()
	base.Mappings[0].Status = StatusNotImplemented
	base.Mappings[0].Reason = "no v2 writer"
	base.Mappings[0].Scope = []string{"uml:Class"}
	rendered := renderGaps(base)
	if !strings.Contains(rendered, "| 1 | 2: `Alpha_Mapping`, `Beta_Mapping` |") {
		t.Fatalf("the two shared-scope rows must group into one entry:\n%s", rendered)
	}
	root := testRoot(t, base)
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(content), "| 1 | 2: `Alpha_Mapping`, `Beta_Mapping` |", "| 1 | 3: `Alpha_Mapping`, `Beta_Mapping` |", 1)
	if stale == string(content) {
		t.Fatal("the generated gaps row was not found to edit")
	}
	if err := os.WriteFile(docPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	runCheckFails(t, root, "a generated block is stale")
}

// TestMeasureSeedsAbsentTokens: a scope token counted in neither corpus is
// recorded {0,0}, because validate requires a count for every token used.
func TestMeasureSeedsAbsentTokens(t *testing.T) {
	checkout, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// The measure reads the PSSM pin, one suite file and the fixture roots.
	for _, rel := range []string{"scripts/pssm-pin.sh"} {
		content, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stub := `<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><uml:Model xmi:type="uml:Model" xmlns:uml="http://www.omg.org/spec/UML/20131001"/></xmi:XMI>`
	for _, rel := range []string{pssmSuiteRel, vehicleFixture, "tests/migrate/testdata/xmi/one.xmi"} {
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(stub), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := testBaseline()
	base.Mappings[0].Scope = []string{"uml:Model"}
	base.Mappings[1].Scope = []string{"uml:NonexistentClass"}
	if err := measure(root, base, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := base.Measurement.Counts["uml:NonexistentClass"]; got != (TokenCount{}) {
		t.Fatalf("an absent token must record {0,0}, got %+v", got)
	}
	if got := base.Measurement.Counts["uml:Model"]; got != (TokenCount{PSSM: 1, Fixtures: 2}) {
		t.Fatalf("uml:Model must count once per corpus, got %+v", got)
	}
	if err := base.validate(); err != nil {
		t.Fatalf("the measured baseline must validate: %v", err)
	}
}

// TestCheckCatchesSourceCountDrift: the recorded model dimensions, the OCL
// specification count among them, are compared against a fresh extraction
// whenever the model is provisioned.
func TestCheckCatchesSourceCountDrift(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := ReadPin(root)
	if err != nil {
		t.Fatal(err)
	}
	xmi := modelPath(root, pin.File, "")
	if _, err := os.Stat(xmi); err != nil {
		t.Skipf("pinned model not provisioned at %s (run ./scripts/download-sysml-v1tov2.sh)", xmi)
	}
	base, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	base.Source.OCLSpecifications++
	if err := compareXMI(root, base, options{}, &bytes.Buffer{}); err == nil {
		t.Fatal("a drifted oclSpecifications count must fail")
	}
	base, err = loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareXMI(root, base, options{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("the committed baseline must compare: %v", err)
	}
}
