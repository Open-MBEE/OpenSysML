package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// classifyFixture parses grammar snippets, writes corpus files and classifies,
// which is the whole pipeline the reported figures come from.
func classifyFixture(t *testing.T, corpus map[string]string, sources ...string) map[string]Row {
	t.Helper()
	repo := t.TempDir()
	for name, content := range corpus {
		path := filepath.Join(repo, "models", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var grammars []*Grammar
	var literals []string
	for i, src := range sources {
		grammar, err := ParseGrammar(grammarName(i), src)
		if err != nil {
			t.Fatalf("ParseGrammar: %v", err)
		}
		grammars = append(grammars, grammar)
		for _, production := range grammar.Productions {
			literals = append(literals, production.Literals()...)
		}
	}
	lits, err := newLitTable(literals)
	if err != nil {
		t.Fatal(err)
	}
	index, err := buildLiteralIndex(repo, []corpusRoot{{Name: "models", Dir: "models"}}, lits)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := newAnalyzer(grammars, lits)
	rows := map[string]Row{}
	for _, grammar := range grammars {
		for _, production := range grammar.Productions {
			rows[production.Name] = classify(production, analyzer, index)
		}
	}
	return rows
}

func grammarName(i int) string {
	return string(rune('A'+i)) + ".xtext"
}

func TestClassifyBuckets(t *testing.T) {
	const grammar = `
grammar org.example.A

Part returns SysML::Part :
	'part' 'def' Name ';'
;

Port returns SysML::Port :
	'port' 'def' Name ';'
;

Usage returns SysML::Usage :
	Part | Port
;

fragment Identification returns SysML::Element :
	( '<' Name '>' )?
;

terminal Name :
	('a'..'z')+
;
`
	rows := classifyFixture(t, map[string]string{
		"model.sysml": "part def Wheel;\n",
	}, grammar)

	for name, want := range map[string]Bucket{
		"Part":           BucketEvidence,
		"Port":           BucketNoEvidence,
		"Usage":          BucketEvidence,
		"Identification": BucketIndistinguishable,
		"Name":           BucketIndistinguishable,
	} {
		if got := rows[name].Bucket; got != want {
			t.Errorf("%s bucket = %s, want %s (%s)", name, got, want, rows[name].Reason)
		}
	}

	// Evidence names the input and the line each required literal is on.
	part := rows["Part"]
	if part.File != "models/model.sysml" || len(part.Evidence) != 3 {
		t.Fatalf("Part evidence = %+v", part)
	}
	for _, citation := range part.Evidence {
		if citation.Line != 1 || citation.Root != "models" {
			t.Errorf("citation = %+v", citation)
		}
	}
	// A delegating rule is credited with the alternative that has evidence.
	if got := quoteAll(rows["Usage"].Required); got != `";" "def" "part"` {
		t.Errorf("Usage required = %s", got)
	}
	// The missing literal is what makes the no-evidence entry actionable.
	if got := quoteAll(rows["Port"].Missing); got != `"port"` {
		t.Errorf("Port missing = %s, reason %q", got, rows["Port"].Reason)
	}
}

// Literals from two unrelated inputs were never one input, so they are not
// evidence together.
func TestClassifyRequiresOneInput(t *testing.T) {
	const grammar = `
grammar org.example.A

Flow returns SysML::Flow :
	'flow' 'from' 'to' ';'
;
`
	rows := classifyFixture(t, map[string]string{
		"a.sysml": "flow from ;\n",
		"b.sysml": "to ;\n",
	}, grammar)
	row := rows["Flow"]
	if row.Bucket != BucketNoEvidence || row.Reason != reasonNotTogether {
		t.Fatalf("Flow = %s (%s), want no-evidence across files", row.Bucket, row.Reason)
	}
	if len(row.Missing) != 0 {
		t.Errorf("Flow missing = %s, want none: every literal occurs somewhere", quoteAll(row.Missing))
	}

	rows = classifyFixture(t, map[string]string{"a.sysml": "flow from to ;\n"}, grammar)
	if got := rows["Flow"].Bucket; got != BucketEvidence {
		t.Errorf("one input with all of them = %s, want evidence", got)
	}
}

// A rule called from another grammar resolves through the `with` chain, and an
// @Override of it wins for the grammar that declares the override.
func TestClassifyResolvesInheritedRules(t *testing.T) {
	const base = `
grammar org.example.A

Keyword returns SysML::Element :
	'occurrence'
;

Definition returns SysML::Element :
	Keyword 'def' ';'
;
`
	const derived = `
grammar org.example.B with org.example.A

@Override
Keyword returns SysML::Element :
	'individual'
;
`
	rows := classifyFixture(t, map[string]string{"a.sysml": "occurrence def ;\n"}, base, derived)
	if got := rows["Definition"].Bucket; got != BucketEvidence {
		t.Errorf("Definition = %s, want evidence through the called rule", got)
	}
	if got := quoteAll(rows["Definition"].Required); got != `";" "def" "occurrence"` {
		t.Errorf("Definition required = %s, want the base grammar's keyword", got)
	}
	if got := rows["Keyword"].Bucket; got != BucketNoEvidence {
		t.Errorf("overriding Keyword = %s, want no-evidence for 'individual'", got)
	}
}

// A production is evidence as soon as its cheapest form is present, so the forms
// it does not need are reported separately.
func TestClassifyUnseenForms(t *testing.T) {
	const grammar = `
grammar org.example.A

Disjoining returns SysML::Disjoining :
	( 'disjoining' Name )? 'disjoint' 'from' ';'
;
`
	rows := classifyFixture(t, map[string]string{"a.sysml": "disjoint from ;\n"}, grammar)
	row := rows["Disjoining"]
	if row.Bucket != BucketEvidence {
		t.Fatalf("Disjoining = %s, want evidence", row.Bucket)
	}
	unseen := row.UnseenBranches()
	// The form's own literal, plus what every path through the rule needs anyway.
	if len(unseen) != 1 || quoteAll(unseen[0].Literals) != `";" "disjoining" "disjoint" "from"` {
		t.Fatalf("unseen forms = %+v, want the named form", unseen)
	}
	if got := quoteAll(unseen[0].Missing); got != `"disjoining"` {
		t.Errorf("missing = %s, want only the literal no input has", got)
	}
	for _, branch := range row.Branches {
		if branch.Seen() && !strings.HasPrefix(branch.File, "models/") {
			t.Errorf("branch %+v cites no input", branch)
		}
	}
}

// Left recursion must terminate rather than loop.
func TestClassifyLeftRecursion(t *testing.T) {
	const grammar = `
grammar org.example.A

Expression returns SysML::Expression :
	Primary ( '+' Primary )*
;

Primary returns SysML::Expression :
	'(' Expression ')' | 'null'
;
`
	rows := classifyFixture(t, map[string]string{"a.sysml": "null\n"}, grammar)
	if got := rows["Expression"].Bucket; got != BucketEvidence {
		t.Errorf("Expression = %s, want evidence via the non-recursive branch", got)
	}
}

func TestMaskAndLitTable(t *testing.T) {
	lits, err := newLitTable([]string{"part", "def", "part", ";"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(lits.order), 3; got != want {
		t.Fatalf("literals = %d, want %d deduplicated", got, want)
	}
	both := lits.mask("part").or(lits.mask("def"))
	if !lits.mask("part").subsetOf(both) || both.subsetOf(lits.mask("part")) {
		t.Errorf("subsetOf is wrong for %v", both)
	}
	if got := both.count(); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
	if got := quoteAll(lits.names(both)); got != `"def" "part"` {
		t.Errorf("names = %s", got)
	}
	if got := both.andNot(lits.mask("part")); !got.subsetOf(lits.mask("def")) || got.isZero() {
		t.Errorf("andNot = %v, want just def", got)
	}
	// An unknown literal is not part of any set, so it cannot fake evidence.
	if !lits.mask("nope").isZero() {
		t.Errorf("unknown literal has a bit")
	}
}

// A path set keeps only the cheapest requirements: a set containing another is
// satisfied only when the smaller one is.
func TestPruneDropsRedundantPaths(t *testing.T) {
	lits, err := newLitTable([]string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	a := lits.mask("a")
	ab := a.or(lits.mask("b"))
	c := lits.mask("c")
	got := prune([]mask{ab, c, a, ab})
	if len(got) != 2 || !got[0].subsetOf(a.or(c)) || got[1].count() != 1 {
		t.Fatalf("prune = %v, want the two single-literal sets", got)
	}
}
