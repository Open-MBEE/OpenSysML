package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// otherContent returns the rewritten content of the other document name, failing
// when the result does not rewrite it.
func otherContent(t *testing.T, res *Result, name string) string {
	t.Helper()
	for _, other := range res.Others {
		if other.Name == name {
			return string(other.Content)
		}
	}
	t.Fatalf("result rewrites %v, not %s", otherNames(res), name)
	return ""
}

func otherNames(res *Result) []string {
	names := make([]string, 0, len(res.Others))
	for _, other := range res.Others {
		names = append(names, other.Name)
	}
	return names
}

// Renaming a declaration respells every reference written with its name in the
// other documents the edit may rewrite, the way textDocument/rename does: one
// written with the short name or through an alias is left alone, and an alias
// declared for the target is respelled.
func TestRenameRespellsReferencesInOtherDocuments(t *testing.T) {
	const src = "package P {\n    part def <O> Old;\n    part def Keep;\n}\n"
	q := "package Q {\n    private import P::Old;\n    part a : P::Old;\n    part b : P::O;\n}\n"
	r := "package R {\n    alias Alt for P::Old;\n    part c : Alt;\n    part k : P::Keep;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src, map[string]string{"q.sysml": q, "r.sysml": r})
	requireClean(t, m)

	res := applyOne(t, m, Rename("P::Old", "Fresh"))
	assertOnlySpanChanged(t, m, res)
	if got, want := string(res.Content), "package P {\n    part def <O> Fresh;\n    part def Keep;\n}\n"; got != want {
		t.Fatalf("p.sysml = %q, want %q", got, want)
	}
	if got := strings.Join(otherNames(res), ","); got != "q.sysml,r.sysml" {
		t.Fatalf("rewritten documents = %s, want q.sysml,r.sysml", got)
	}
	wantQ := "package Q {\n    private import P::Fresh;\n    part a : P::Fresh;\n    part b : P::O;\n}\n"
	if got := otherContent(t, res, "q.sysml"); got != wantQ {
		t.Fatalf("q.sysml = %q, want %q", got, wantQ)
	}
	wantR := "package R {\n    alias Alt for P::Fresh;\n    part c : Alt;\n    part k : P::Keep;\n}\n"
	if got := otherContent(t, res, "r.sysml"); got != wantR {
		t.Fatalf("r.sysml = %q, want %q", got, wantR)
	}
	for i, other := range res.Others {
		if want := 2 - i; len(other.Applied) != want {
			t.Fatalf("%s applied = %+v, want %d ranges", other.Name, other.Applied, want)
		}
		for _, a := range other.Applied {
			if a.Target != "P::Old" || a.OperationIndex != 0 || a.OldText != "Old" || a.NewText != "Fresh" {
				t.Fatalf("%s applied = %+v, want operation 0 on P::Old", other.Name, a)
			}
		}
	}
	if len(res.Applied) != 1 {
		t.Fatalf("p.sysml applied = %+v, want the declaration alone", res.Applied)
	}
}

// Documents with no reference to the target are not rewritten, so a model whose
// other documents never mention it gives the one-document result.
func TestRenameLeavesUnreferencingDocumentsAlone(t *testing.T) {
	m := loadEditableWorkspace(t, "p.sysml", "package P {\n    part def Old;\n}\n",
		map[string]string{"q.sysml": "package Q {\n    part def Other;\n}\n"})
	requireClean(t, m)
	res := applyOne(t, m, Rename("P::Old", "Fresh"))
	if len(res.Others) != 0 {
		t.Fatalf("rewritten documents = %v, want none", otherNames(res))
	}
	if got, want := string(res.Content), "package P {\n    part def Fresh;\n}\n"; got != want {
		t.Fatalf("p.sysml = %q, want %q", got, want)
	}
}

// A reference from a document the edit may not rewrite refuses the rename even
// when others may be followed, and the refusal names only the referrers that
// could not follow.
func TestRenameRefusesForTheDocumentsItMayNotRewrite(t *testing.T) {
	const src = "package P {\n    part def Old;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src,
		map[string]string{"q.sysml": "package Q {\n    part a : P::Old;\n}\n"})
	m.Index.AddDocument("locked.sysml", parseOnly("locked.sysml", "package L {\n    part x : P::Old;\n}\n"))
	m.Index.ExpandWildcardImports()

	e := addFailure(t, m, Rename("P::Old", "Fresh"), FailureReferencedElsewhere)
	if got := strings.Join(e.Referring, ","); got != "L::x (locked.sysml)" {
		t.Fatalf("referring = %v, want L::x (locked.sysml)", e.Referring)
	}
	if len(e.Referrers) != 1 || e.Referrers[0] != (Referrer{Name: "L::x", Document: "locked.sysml"}) {
		t.Fatalf("referrers = %+v, want L::x in locked.sysml", e.Referrers)
	}
}

// A rename that another document's names would capture is refused, and nothing
// is rewritten anywhere.
func TestRenameConflictInAnotherDocumentRefusesWhole(t *testing.T) {
	const src = "package P {\n    part def Old;\n}\n"
	q := "package Q {\n    private import P::*;\n    part def Fresh;\n    part a : Old;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src, map[string]string{"q.sysml": q})
	requireClean(t, m)
	e := addFailure(t, m, Rename("P::Old", "Fresh"), FailureInvalidName)
	if !strings.Contains(e.Message, "Fresh") {
		t.Fatalf("message %q does not name the conflict", e.Message)
	}
}

// Errors another document already had do not refuse an edit that leaves them as
// they were: validation compares each document with its own original.
func TestEditToleratesPreexistingErrorsInAnotherDocument(t *testing.T) {
	const src = "package P {\n    part def Old;\n}\n"
	q := "package Q {\n    part a : P::Old;\n    part broken : Missing;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src, map[string]string{"q.sysml": q})
	res := applyOne(t, m, Rename("P::Old", "Fresh"))
	want := "package Q {\n    part a : P::Fresh;\n    part broken : Missing;\n}\n"
	if got := otherContent(t, res, "q.sysml"); got != want {
		t.Fatalf("q.sysml = %q, want %q", got, want)
	}
}

// A cascade delete removes the referrers in other documents and their referrers
// in turn, across documents, and leaves the rest of each document alone.
func TestDeleteCascadeFollowsReferrersAcrossDocuments(t *testing.T) {
	const src = "package P {\n    part def Base;\n    part def Keep;\n}\n"
	q := "package Q {\n    part b : P::Base;\n    part k : P::Keep;\n}\n"
	r := "package R {\n    part c = Q::b;\n    part d : P::Keep;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src, map[string]string{"q.sysml": q, "r.sysml": r})
	requireClean(t, m)

	res := applyOne(t, m, Delete("P::Base", true))
	if got, want := string(res.Content), "package P {\n    part def Keep;\n}\n"; got != want {
		t.Fatalf("p.sysml = %q, want %q", got, want)
	}
	if got, want := otherContent(t, res, "q.sysml"), "package Q {\n    part k : P::Keep;\n}\n"; got != want {
		t.Fatalf("q.sysml = %q, want %q", got, want)
	}
	if got, want := otherContent(t, res, "r.sysml"), "package R {\n    part d : P::Keep;\n}\n"; got != want {
		t.Fatalf("r.sysml = %q, want %q", got, want)
	}
}

// Without cascade the delete is refused, and the referrers of other documents
// are named with their document so a client can list them by file.
func TestDeleteWithoutCascadeNamesReferrersInOtherDocuments(t *testing.T) {
	const src = "package P {\n    part def Base;\n    part own : Base;\n}\n"
	q := "package Q {\n    part b : P::Base;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src, map[string]string{"q.sysml": q})
	requireClean(t, m)

	e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
	if got := strings.Join(e.Referring, ","); got != "P::own,Q::b (q.sysml)" {
		t.Fatalf("referring = %v, want P::own and Q::b (q.sysml)", e.Referring)
	}
	want := []Referrer{{Name: "P::own", Document: "p.sysml"}, {Name: "Q::b", Document: "q.sysml"}}
	if len(e.Referrers) != len(want) || e.Referrers[0] != want[0] || e.Referrers[1] != want[1] {
		t.Fatalf("referrers = %+v, want %+v", e.Referrers, want)
	}
}

// A cascade reaching a document the edit may not rewrite is refused as a whole,
// naming what could not follow, even though the other documents could have been.
func TestDeleteCascadeRefusesForTheDocumentsItMayNotRewrite(t *testing.T) {
	const src = "package P {\n    part def Base;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src,
		map[string]string{"q.sysml": "package Q {\n    part b : P::Base;\n}\n"})
	m.Index.AddDocument("locked.sysml", parseOnly("locked.sysml", "package L {\n    part x = Q::b;\n}\n"))
	m.Index.ExpandWildcardImports()

	e := addFailure(t, m, Delete("P::Base", true), FailureReferencedElsewhere)
	if got := strings.Join(e.Referring, ","); got != "L::x (locked.sysml)" {
		t.Fatalf("referring = %v, want L::x (locked.sysml)", e.Referring)
	}
}

// Operations of one request build on each other across documents: the second
// sees the other document as the first left it.
func TestOperationsRewriteOtherDocumentsInSequence(t *testing.T) {
	const src = "package P {\n    part def Old;\n    part def Gone;\n}\n"
	q := "package Q {\n    part a : P::Old;\n    part g : P::Gone;\n}\n"
	m := loadEditableWorkspace(t, "p.sysml", src, map[string]string{"q.sysml": q})
	requireClean(t, m)

	res, err := Apply(m, []Operation{Rename("P::Old", "Fresh"), Delete("P::Gone", true)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := string(res.Content), "package P {\n    part def Fresh;\n}\n"; got != want {
		t.Fatalf("p.sysml = %q, want %q", got, want)
	}
	if got, want := otherContent(t, res, "q.sysml"), "package Q {\n    part a : P::Fresh;\n}\n"; got != want {
		t.Fatalf("q.sysml = %q, want %q", got, want)
	}
	other := res.Others[0]
	if len(other.Applied) != 2 || other.Applied[0].OperationIndex != 0 || other.Applied[1].OperationIndex != 1 {
		t.Fatalf("q.sysml applied = %+v, want one range per operation", other.Applied)
	}
}

// parseOnly parses a document indexed but not handed out for rewriting, as one
// the language server does not hold.
func parseOnly(name, content string) *ast.RootNamespace {
	return parser.New(source.New(name, []byte(content))).ParseFile()
}
