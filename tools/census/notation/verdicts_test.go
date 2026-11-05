package notation

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEveryExcerptProductionHasAVerdict(t *testing.T) {
	g, err := Extract(readExcerpt(t))
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ReadNotationTable(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[Verdict]int{}
	for _, p := range g.Productions {
		adj, err := Adjudicate(p.Name, edges)
		if err != nil {
			t.Error(err)
			continue
		}
		if !knownVerdict(adj.Verdict) || !knownCategory(adj.Category) {
			t.Errorf("%s: verdict %q category %q", p.Name, adj.Verdict, adj.Category)
		}
		if adj.Verdict != VerdictDrawn && adj.Reason == "" {
			t.Errorf("%s is %s without a reason", p.Name, adj.Verdict)
		}
		counts[adj.Verdict]++
	}
	if counts[VerdictDrawn] == 0 || counts[VerdictUndrawn] == 0 {
		t.Errorf("verdicts %v", counts)
	}
	for _, name := range []string{"no-such-production", "symbols-compartment-element", "symbols-compartment-contents"} {
		if _, err := Adjudicate(name, edges); err == nil || !strings.Contains(err.Error(), "no adjudication") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAdjudicationRules(t *testing.T) {
	edges, err := ReadNotationTable(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		verdict  Verdict
		category Category
	}{
		{"parts-compartment-contents", VerdictNotApplicable, CategoryTextual},
		{"parts-compartment-element", VerdictNotApplicable, CategoryTextual},
		{"exhibit-states-compartment-element-compartment", VerdictNotApplicable, CategoryTextual},
		{"usage-node", VerdictNotApplicable, CategoryNode},
		{"part-def", VerdictDrawn, CategoryNode},
		{"parts-compartment", VerdictDrawn, CategoryCompartment},
		{"calcs-compartment", VerdictDrawn, CategoryCompartment},
		{"connection-graphical", VerdictDrawn, CategoryEdge},
		{"connection-def-graphical", VerdictDrawn, CategoryEdge},
		{"flow", VerdictApproximate, CategoryEdge},
		{"aflow-succession", VerdictDrawn, CategoryEdge},
		{"n-ary-connection-dot", VerdictDrawn, CategoryNode},
		{"general-view", VerdictDrawn, CategoryView},
		{"root-view", VerdictDrawn, CategoryView},
		{"connection-definition-elaboration", VerdictUndrawn, CategoryView},
	}
	for _, tc := range cases {
		adj, err := Adjudicate(tc.name, edges)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if adj.Verdict != tc.verdict || adj.Category != tc.category {
			t.Errorf("%s: %s %s, want %s %s", tc.name, adj.Verdict, adj.Category, tc.verdict, tc.category)
		}
	}
}

func TestParseNotationTable(t *testing.T) {
	content := "intro\n\n### Notation table\n\nprose\n\n" +
		"| Production | `EdgeKind` | DOT | PlantUML | Mermaid | D2 | draw.io | Canvas | Status | Notes |\n" +
		"| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n" +
		"| `connection` | `connection` | `arrowhead=none` | `--` | `---` | `--` | `endArrow=none` | `none` | conformant | |\n" +
		"| `subclassification`, `specialization` | `specialization` | `arrowhead=empty` | `--\\|>` | `-->` | `->` | `endArrow=block` | `end triangle-hollow` | approximate | Mermaid has no triangle head ([glyphs](#glyphs)). |\n" +
		"| objective anchor | `anchor` | a | b | c | d | e | f | conformant | |\n" +
		"\n### Next\n\n| not | a | notation | table | row | at | all | so | it | stops |\n"
	table, err := ParseNotationTable(content)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(table.Names(), ","); got != "connection-graphical,subclassification" {
		t.Errorf("names %q", got)
	}
	if adj, ok := table.adjudicate("subclassification"); !ok || adj.Verdict != VerdictApproximate || !strings.Contains(adj.Reason, "Mermaid has no triangle head ([glyphs](view-rendering-forms.md#glyphs))") {
		t.Errorf("subclassification: %v %+v", ok, adj)
	}
	if adj, ok := table.adjudicate("connection-graphical"); !ok || adj.Verdict != VerdictDrawn {
		t.Errorf("connection-graphical: %v %+v", ok, adj)
	}
	if _, err := ParseNotationTable("no table here"); err == nil {
		t.Error("a document without the section parses")
	}
	if _, err := ParseNotationTable("### Notation table\n| a | b |\n"); err == nil {
		t.Error("a row with the wrong cell count parses")
	}
	misspelt := strings.Replace(content, "| approximate |", "| approximte |", 1)
	if _, err := ParseNotationTable(misspelt); err == nil || !strings.Contains(err.Error(), "states status \"approximte\"") {
		t.Errorf("a misspelt status: %v", err)
	}
}

func TestCheckEvidenceResolvesTheTable(t *testing.T) {
	root := repoRoot(t)
	var rows []Row
	for _, name := range AdjudicatedNames() {
		rows = append(rows, Row{Production: Production{Name: name}, Adjudication: table[name]})
	}
	if problems := CheckEvidence(root, rows); len(problems) > 0 {
		t.Fatalf("the verdict table's evidence does not resolve:\n%s", strings.Join(problems, "\n"))
	}
}

func TestCheckEvidenceRejectsBadCitations(t *testing.T) {
	root := repoRoot(t)
	ev := Evidence{"internal/ir/view/label.go:keyword", goldenForms, "internal/ir/view/label_test.go:TestLabelLines"}
	rows := []Row{
		{Production{Name: "a"}, row(CategoryNode, VerdictDrawn, "", Evidence{"internal/ir/view/label.go:noSuchFunction", goldenForms, ev.Test})},
		{Production{Name: "b"}, row(CategoryNode, VerdictDrawn, "", Evidence{ev.Implementation, goldenForms, "internal/ir/view/label_test.go:TestNoSuchTest"})},
		{Production{Name: "c"}, row(CategoryNode, VerdictDrawn, "", Evidence{ev.Implementation, []string{"Visio"}, ev.Test})},
		{Production{Name: "d"}, row(CategoryNode, VerdictDrawn, "", Evidence{ev.Implementation, goldenForms, "internal/ir/view/testdata/no-such.dot.golden"})},
		{Production{Name: "e"}, row(CategoryNode, VerdictApproximate, "why", Evidence{})},
		{Production{Name: "f"}, row(CategoryNode, VerdictUndrawn, "why", ev)},
		{Production{Name: "g"}, row(CategoryNode, VerdictDrawn, "", Evidence{ev.Test, goldenForms, ev.Test})},
		{Production{Name: "h"}, row(CategoryNode, VerdictDrawn, "", Evidence{"editors/vscode/src/webview/canvas.ts:noSuchFunction", []string{FormCanvas}, ev.Test})},
		{Production{Name: "i"}, row(CategoryNode, VerdictDrawn, "", Evidence{"editors/vscode/src/webview/canvas.ts:drawEdge", []string{FormCanvas}, ev.Test})},
		{Production{Name: "j"}, row(CategoryNode, VerdictDrawn, "", ev)},
	}
	problems := CheckEvidence(root, rows)
	for _, want := range []string{"a: internal/ir/view/label.go declares no function noSuchFunction", "b: internal/ir/view/label_test.go declares no function TestNoSuchTest",
		"c: unknown form \"Visio\"", "d: ", "e: a approximate row needs an implementation, forms and a test", "f: a undrawn row carries evidence",
		"g: ", "h: editors/vscode/src/webview/canvas.ts declares no function noSuchFunction"} {
		found := false
		for _, p := range problems {
			if strings.HasPrefix(p, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem starting %q in\n%s", want, strings.Join(problems, "\n"))
		}
	}
	for _, p := range problems {
		if strings.HasPrefix(p, "i:") || strings.HasPrefix(p, "j:") {
			t.Errorf("a resolving citation is rejected: %s", p)
		}
	}
}
