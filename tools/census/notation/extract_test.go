package notation

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// excerptPath is the committed excerpt of the rendered specification: every
// §8.2.3 heading and production statement of the pinned text, each statement
// cut after its `=`, with its page-start form feeds and a few page footers.
const excerptPath = "testdata/sysml-8.2.3-productions.txt"

func readExcerpt(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(excerptPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestExtractTheCommittedExcerpt(t *testing.T) {
	g, err := Extract(readExcerpt(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Clauses) != 27 || len(g.Statements) != 620 || len(g.Productions) != 523 || g.Partials() != 97 {
		t.Fatalf("clauses %d statements %d productions %d partials %d; want 27, 620, 523, 97",
			len(g.Clauses), len(g.Statements), len(g.Productions), g.Partials())
	}
	if g.Clauses[0].Statements != 0 || g.Clauses[0].Title != "Graphical Notation Overview" {
		t.Errorf("overview: %+v", g.Clauses[0])
	}
	annotations := g.Clauses[3]
	if annotations.Number != "8.2.3.4" || annotations.Title != "Annotations Graphical Notation" || annotations.Statements != 12 {
		t.Errorf("the form-feed-prefixed §8.2.3.4 heading: %+v", annotations)
	}
	byName := map[string]Production{}
	for _, p := range g.Productions {
		byName[p.Name] = p
	}
	if p := byName["ellipsis-at-lower-left-corner"]; p.Clause != "8.2.3.2" {
		t.Errorf("a statement at the start of a page is kept: %+v", p)
	}
	if p := byName["compartment"]; !p.Partial || p.Statements != 24 || len(p.Clauses) < 20 {
		t.Errorf("compartment is a partial production extended in most clauses: %+v", p)
	}
	if p := byName["usage-node"]; p.Partial || p.Statements != 19 {
		t.Errorf("usage-node has a complete statement among its partial ones: %+v", p)
	}
	if p := byName["comment-node"]; p.Clause != "8.2.3.4" || p.Partial || p.Clauses != nil {
		t.Errorf("comment-node: %+v", p)
	}
}

func TestExtractIsDeterministic(t *testing.T) {
	text := readExcerpt(t)
	a, err := Extract(text)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Extract(text)
	if strings.Join(a.ProductionNames(), ",") != strings.Join(b.ProductionNames(), ",") || len(a.Statements) != len(b.Statements) {
		t.Fatal("two extractions differ")
	}
}

func TestExtractRejectsAChangedText(t *testing.T) {
	text := readExcerpt(t)
	cases := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"no overview heading", func(s string) string { return strings.Replace(s, overviewTitle, "8.2.3.1 Overview", 1) }, "no \"8.2.3.1 Graphical Notation Overview\" heading"},
		{"no next clause", func(s string) string { return strings.Replace(s, nextClauseHead, "8.3 Abstract", 1) }, "no \"8.3 Abstract Syntax\" heading"},
		{"a clause heading lost", func(s string) string { return strings.Replace(s, "\f8.2.3.4 Annotations Graphical Notation", "", 1) }, "clause 8.2.3.5 follows 3 sub-clauses"},
		{"a clause without productions", func(s string) string {
			return strings.Replace(s, "8.2.3.22 Cases Graphical Notation\ncompartment =|\nobjective-compartment =\nobjective-compartment-contents =\nobjective-compartment-element =\n", "8.2.3.22 Cases Graphical Notation\n", 1)
		}, "8.2.3.22 Cases Graphical Notation states no production"},
		{"a production in the overview", func(s string) string {
			return strings.Replace(s, overviewTitle+"\n", overviewTitle+"\nearly-node =\n", 1)
		}, "8.2.3.1 Graphical Notation Overview states a production"},
		{"a heading out of order", func(s string) string { return strings.Replace(s, "8.2.3.3 Dependencies", "8.2.3.5 Dependencies", 1) }, "follows 2 sub-clauses"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := tc.mutate(text)
			if mutated == text {
				t.Fatal("the mutation changed nothing")
			}
			_, err := Extract(mutated)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestExtractIgnoresRightHandSidesAndFooters(t *testing.T) {
	text := strings.Join([]string{
		overviewTitle,
		"8.2.3.2 Elements and Relationships Graphical Notation",
		"element = part | item",
		"Systems Modeling Language v2.1, Beta 2, Part 1                          209",
		"\fcompartment =| general-compartment",
		"general-compartment = compartment-label = ignored",
	}, "\n")
	for n := 3; n <= clauseCount; n++ {
		text += "\n8.2.3." + itoa(n) + " Filler Graphical Notation\nfiller-" + itoa(n) + " ="
	}
	text += "\n" + nextClauseHead + "\n"
	g, err := Extract(text)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.ProductionNames()[:3], ","); got != "compartment,element,filler-10" {
		t.Errorf("productions %q", got)
	}
	if g.Clauses[1].Statements != 3 {
		t.Errorf("§8.2.3.2 counts %d statements, want 3", g.Clauses[1].Statements)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestExcerptIsUnderTheFixtureCap(t *testing.T) {
	info, err := os.Stat(filepath.FromSlash(excerptPath))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 200<<10 {
		t.Fatalf("%s is %d bytes, over the tests/hygiene fixture cap", excerptPath, info.Size())
	}
}
