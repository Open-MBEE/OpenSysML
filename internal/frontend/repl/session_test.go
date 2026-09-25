package repl

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestNewSessionEmpty(t *testing.T) {
	s := NewSession()
	if s == nil {
		t.Fatal("NewSession returned nil")
	}
	if got := s.List(); len(got) != 0 {
		t.Fatalf("new session should have no declarations, got %v", got)
	}
}

func TestAcceptReplacesByName(t *testing.T) {
	s := NewSession()
	s.accept("", "package P { }")
	s.accept("", "namespace N;")
	s.accept("", "package P { } // redefined")
	joined := s.joined()
	// P should appear once (the new one); N preserved; order = N then new P.
	if got := s.List(); len(got) != 2 {
		t.Fatalf("want 2 snippets, got %d: %v", len(got), got)
	}
	if !strings.Contains(joined, "redefined") {
		t.Fatalf("joined missing new P: %q", joined)
	}
	if strings.Count(joined, "package P") != 1 {
		t.Fatalf("P not deduplicated: %q", joined)
	}
}

// Re-typing a namespace to add a member adds to the one already in the buffer:
// a REPL user builds a package up a line at a time, so a second body is an
// addition, not a replacement of everything typed before it.
func TestResubmittedNamespaceMergesItsBody(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	res := s.Submit("package P { part def B; }")

	list := strings.Join(s.List(), "\n")
	wants(t, list, "part def A", "part def B")
	if got := len(s.List()); got != 1 {
		t.Errorf("want one snippet standing for the merged package, got %d: %v", got, s.List())
	}
	if !hasNotice(res, "added to the existing package P") {
		t.Errorf("merge was not reported: %v", res.Notices)
	}
}

// A member the new body redeclares still replaces the old one — and is named,
// so the definition that was superseded is not lost silently.
func TestMergedNamespaceReportsTheMembersItReplaces(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A { attribute x = 1; } part def C; }")
	res := s.Submit("package P { part def A; }")

	list := strings.Join(s.List(), "\n")
	wants(t, list, "part def C")
	rejects(t, list, "attribute x = 1")
	if !hasNotice(res, "replacing part def A") {
		t.Errorf("replaced member was not reported: %v", res.Notices)
	}
}

// Merging must not deduplicate what one submission declares twice: the buffer
// keeps both, so the submission is still a duplicate declaration.
func TestMergeKeepsDuplicateDeclarationsOfOneSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def C; }")
	s.Submit("package P { part def A; part def A; }")

	if got := strings.Count(strings.Join(s.List(), "\n"), "part def A"); got != 2 {
		t.Errorf("want both duplicate declarations kept, got %d: %v", got, s.List())
	}
}

// Nesting merges too, so adding to a nested package keeps the members of both
// it and its parent.
func TestNestedNamespaceMergeKeepsNestedMembers(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def Top; package Q { part def A; } }")
	s.Submit("package P { package Q { part def B; } }")

	wants(t, strings.Join(s.List(), "\n"), "part def Top", "part def A", "part def B")
}

// An empty body is how a namespace is emptied, so it still replaces — and says
// what it dropped.
func TestEmptyBodyReplacesNamespaceWithANotice(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	res := s.Submit("package P { }")

	rejects(t, strings.Join(s.List(), "\n"), "part def A")
	if !hasNotice(res, "replaced package P (part def A no longer declared)") {
		t.Errorf("replacement was not reported: %v", res.Notices)
	}
}

// A merge absorbs the whole snippet the namespace sat in, so the declarations
// that shared it stay replaceable by name instead of being duplicated.
func TestMergeKeepsTrackingTheAbsorbedSnippetsNames(t *testing.T) {
	s := NewSession()
	s.Submit("part def Z; package P { part def A; }")
	s.Submit("package P { part def B; }")
	s.Submit("part def Z { attribute q = 1; }")

	if got := strings.Count(strings.Join(s.List(), "\n"), "part def Z"); got != 1 {
		t.Errorf("Z declared %d times after being absorbed by a merge: %v", got, s.List())
	}
}

// A member declaring no name is matched by its text, so re-typing a body does
// not stack another copy of its imports and comments.
func TestMergeDoesNotDuplicateUnnamedMembers(t *testing.T) {
	s := NewSession()
	const src = "package P { public import ScalarValues::*; part def A; }"
	s.Submit(src)
	s.Submit(src)
	s.Submit(src)

	if got := strings.Count(strings.Join(s.List(), "\n"), "import ScalarValues"); got != 1 {
		t.Errorf("import present %d times after re-typing the body: %v", got, s.List())
	}
}

// The text a member is matched by ends at the member, so a comment written
// after an import does not make re-typing the body stack another copy of it.
func TestMergeDoesNotDuplicateAnImportFollowedByAComment(t *testing.T) {
	s := NewSession()
	const src = "package P {\n\tpublic import ScalarValues::*; // std\n\tpart def A;\n}"
	s.Submit(src)
	s.Submit(src)

	if got := strings.Count(strings.Join(s.List(), "\n"), "import ScalarValues"); got != 1 {
		t.Errorf("import present %d times after re-typing the body: %v", got, s.List())
	}
}

// A member added to a nested body lines up with the members already in it.
func TestNestedMergeIndentsTheAddedMember(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpackage Q {\n\t\tpart def A;\n\t}\n}")
	s.Submit("package P { package Q { part def B; } }")

	wants(t, strings.Join(s.List(), "\n"), "\t\tpart def A;\n\t\tpart def B;\n")
}

// Redeclaring the last member of an indented nested body keeps the new text: the
// deletion of the old member must not swallow the point the new one goes at.
func TestNestedMergeReplacesTheLastMemberOfABody(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpackage Q {\n\t\tpart def A;\n\t}\n}")
	s.Submit("package P { package Q { part def A { attribute x = 1; } } }")

	got := strings.Join(s.List(), "\n")
	wants(t, got, "part def A { attribute x = 1; }")
	if strings.Contains(got, "\n\n") || strings.Contains(got, "\t\n") {
		t.Errorf("replaced member left a blank line behind:\n%s", got)
	}
}

// Comments inside a body document the member below them, so replacing the
// member above must not take them with it.
func TestMergeKeepsCommentsInsideTheBody(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpart def A;\n\t// doc for B\n\tpart def B;\n}")
	s.Submit("package P { part def A { attribute x = 1; } }")

	wants(t, strings.Join(s.List(), "\n"), "// doc for B")
}

// A re-typed declaration only merges into one written the same way: a different
// header is a different declaration and replaces the old one.
func TestDifferentHeaderReplacesRatherThanMerges(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	res := s.Submit("namespace P { part def B; }")

	got := strings.Join(s.List(), "\n")
	wants(t, got, "namespace P")
	if strings.Contains(got, "package P") {
		t.Errorf("kept the package header the user replaced with a namespace:\n%s", got)
	}
	if !hasNotice(res, "replaced package P") {
		t.Errorf("did not report replacing the package: %v", res.Notices)
	}
}

// An empty body clears a nested namespace as it clears a top-level one.
func TestEmptyNestedBodyClearsTheNestedNamespace(t *testing.T) {
	s := NewSession()
	s.Submit("package P { package Q { part def A; } }")
	res := s.Submit("package P { package Q { } }")

	if got := strings.Join(s.List(), "\n"); strings.Contains(got, "part def A") {
		t.Errorf("emptying package Q kept its members:\n%s", got)
	}
	if !hasNotice(res, "package Q") {
		t.Errorf("did not report what emptying package Q dropped: %v", res.Notices)
	}
}

// Re-typing a body that adds nothing still confirms the declaration it merged
// into, and nothing else that is already in the buffer.
func TestMergeAddingNothingConfirmsOnlyItsDeclaration(t *testing.T) {
	s := NewSession()
	s.Submit("part def Z; package P { public import ScalarValues::*; }")
	res := s.Submit("package P { public import ScalarValues::*; }")

	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "✓ package P")
	if strings.Contains(out, "part def Z") {
		t.Errorf("re-announced a declaration from the absorbed snippet:\n%s", out)
	}
}

// A merge only announces the declaration it added to, and only reports the
// diagnostics of the added text: the rest of the merged snippet was typed
// earlier and has already been reported.
func TestMergeReportsOnlyWhatWasTyped(t *testing.T) {
	s := NewSession()
	s.Submit("part def Z; package P { part def A; }")
	s.Submit("part def Bad : Missing;")
	res := s.Submit("package P { part def B; }")

	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	if strings.Contains(out, "part def Z") {
		t.Errorf("re-announced a declaration from the absorbed snippet:\n%s", out)
	}
	wants(t, out, "✓ package P")
	for _, d := range res.Diagnostics {
		if res.mine(d.Span) {
			t.Errorf("diagnostic outside the added text claimed as this submission's: %v", d)
		}
	}
}

// Comments typed above an addition document the declaration it merges into, so
// they join the ones already above it rather than the end of the buffer.
func TestMergeKeepsCommentsAboveTheDeclaration(t *testing.T) {
	s := NewSession()
	s.Submit("// doc for P")
	s.Submit("package P { part def A; }")
	s.Submit("// more about P")
	s.Submit("package P { part def B; }")

	buf := strings.Join(s.List(), "\n")
	docAt, moreAt := strings.Index(buf, "// doc for P"), strings.Index(buf, "// more about P")
	if docAt < 0 || moreAt < 0 || docAt > moreAt || moreAt > strings.Index(buf, "package P") {
		t.Errorf("comments are not in order above the declaration:\n%s", buf)
	}
}

// A re-declaration that leaves its body open drops nothing: it is not accepted
// into the buffer at all, so what it would have replaced is still declared and
// the submission is reported as the syntax error it is.
func TestUnparseableRedeclarationDropsNothing(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	res := s.Submit("package P { part def B;")

	if len(res.Notices) != 0 {
		t.Errorf("notices = %v, want nothing lost", res.Notices)
	}
	if !hasSyntaxError(res) {
		t.Errorf("diagnostics = %v, want the syntax error reported", res.Diagnostics)
	}
	if _, _, err := s.lookupSymbol("P::A"); err != nil {
		t.Errorf("P::A should still be declared: %v", err)
	}
}

func TestSubmitResolvesAcrossSubmissions(t *testing.T) {
	s := NewSession()
	r1 := s.Submit("package P { }")
	if len(r1.Diagnostics) != 0 {
		t.Fatalf("clean package should have no diags, got %v", r1.Diagnostics)
	}
	if len(r1.Declared) != 1 || r1.Declared[0] != "P" {
		t.Fatalf("want declared [P], got %v", r1.Declared)
	}
	// A later submission referencing an undefined name yields a diagnostic.
	r2 := s.Submit("namespace N { import Missing::X; }")
	if len(r2.Diagnostics) == 0 {
		t.Fatalf("expected unresolved-reference diagnostic")
	}
}

// A comment typed on its own line documents the declaration that follows, so
// redeclaring that declaration replaces the comment with it instead of leaving
// stale documentation above whatever is current.
func TestLeadingCommentIsReplacedWithItsDeclaration(t *testing.T) {
	s := NewSession()
	s.accept("", "// doc for A")
	s.accept("", "part def A;")
	s.accept("", "part def A { part y; }")
	joined := s.joined()

	if strings.Contains(joined, "doc for A") {
		t.Errorf("stale comment survived the redeclaration: %q", joined)
	}
	if got := len(s.List()); got != 1 {
		t.Errorf("want 1 snippet, got %d: %v", got, s.List())
	}
}

// A comment with nothing after it is still part of the session and still saved.
func TestTrailingCommentIsKept(t *testing.T) {
	s := NewSession()
	s.accept("", "part def A;")
	s.accept("", "// thinking out loud")
	joined := s.joined()
	if !strings.Contains(joined, "thinking out loud") {
		t.Errorf("comment dropped: %q", joined)
	}
}

// A comment folded into the declaration below it must not shift the line
// numbers of that declaration's diagnostics.
func TestSubmitReportsTheSubmittedLine(t *testing.T) {
	s := NewSession()
	s.Submit("// doc for A")
	res := s.Submit("part def A { part x : Missing; }")
	sf := source.New(docName, []byte(res.Source))
	for _, d := range res.Diagnostics {
		line := sf.Lines().PosAt(d.Span.Offset).Line - res.baseLine() + 1
		if line != 1 {
			t.Errorf("diagnostic reported on line %d of a one-line submission: %s", line, d.Message)
		}
	}
}

// Several files of one model commonly open the same package, so a loaded file
// supersedes only what that same file contributed before.
func TestLoadedFilesAccumulateByFile(t *testing.T) {
	s := NewSession()
	s.accept("a.sysml", "package M { part def A; }")
	s.accept("b.sysml", "package M { part def B; }")
	if got := len(s.List()); got != 2 {
		t.Errorf("want both files kept, got %d: %v", got, s.List())
	}

	s.accept("a.sysml", "package M { part def C; }")
	joined := s.joined()
	if strings.Contains(joined, "part def A") {
		t.Errorf("reloading a file kept its previous contents: %q", joined)
	}
	if !strings.Contains(joined, "part def B") {
		t.Errorf("reloading a file dropped another file: %q", joined)
	}
}

// A loaded file supersedes what was typed at the prompt about the same names,
// so the name it declares stays unambiguous.
func TestLoadedFileSupersedesPromptDeclarations(t *testing.T) {
	s := NewSession()
	s.accept("", "part def A;")
	s.accept("a.sysml", "part def A { part y; }")
	joined := s.joined()

	if got := len(s.List()); got != 1 {
		t.Errorf("want the typed declaration replaced, got %d snippets: %v", got, s.List())
	}
	if !strings.Contains(joined, "part y") {
		t.Errorf("the loaded declaration is missing: %q", joined)
	}
}

// The same file loaded under another spelling of its path is the same file, so
// it replaces its earlier contents rather than declaring everything twice.
func TestLoadedFileIsIdentifiedByTheFileNotTheSpelling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.sysml")

	s := NewSession()
	s.accept(path, "package M { part def A; }")
	s.accept(filepath.Join(dir, ".", "..", filepath.Base(dir), "m.sysml"), "package M { part def A; }")

	if got := len(s.List()); got != 1 {
		t.Errorf("want one copy of the file, got %d snippets: %v", got, s.List())
	}
}

// A body declaring one name twice is ambiguous, so an addition to that name is
// not folded into each of them.
func TestNestedMergeInsertsOnceWhenTheNameIsDeclaredTwice(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpackage Q { part def A; }\n\tpackage Q { part def B; }\n}")
	s.Submit("package P {\n\tpackage Q { part def C; }\n}")

	if got := strings.Count(strings.Join(s.List(), "\n"), "part def C"); got != 1 {
		t.Errorf("added member present %d times: %v", got, s.List())
	}
}

// A comment after a body's closing brace documents the submission, not the
// declaration, so it does not stop the body being added to.
func TestTrailingCommentStillMerges(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	s.Submit("package P { part def B; } // add B")

	joined := strings.Join(s.List(), "\n")
	for _, want := range []string{"part def A", "part def B"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%s is missing after the merge: %q", want, joined)
		}
	}
}

// A submission of several sources of which one merges still reports the others:
// scoping the report to what a merge added must not hide its siblings.
func TestMultiSourceSubmissionReportsEverySource(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpart def A;\n}")
	res := s.SubmitAll([]string{"package P { part def B; }", "part def Standalone;"})

	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "✓ package P", "✓ part def Standalone")
}

// The same for a diagnostic: an error in the source that did not merge is this
// submission's error, not part of the buffer it was accepted into.
func TestMultiSourceSubmissionReportsASiblingsError(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpart def A;\n}")
	res := s.SubmitAll([]string{"package P { part def B; }", "part def Standalone : Missing;"})

	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "unresolved reference: Missing")
}

// A diagnostic in text merged into an earlier declaration is reported at the
// line it was typed on, not at the line of the declaration it was added to.
func TestMergedDiagnosticKeepsTheSubmittedLine(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n\tpart def A;\n}")
	res := s.Submit("package P { part def Bad : Missing; }")

	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "1:17: error: unresolved reference: Missing")
}

// A loaded file supersedes what was typed about the same names, taking the rest
// of that submission with it, so the load says what it replaced.
func TestLoadingAFileReportsTheTypedDeclarationsItReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.sysml")

	s := NewSession()
	s.Submit("part def Z;\npackage M { part def B; }")
	res := s.submitFiles([]SourceFile{{Name: path, Text: "package M { part def A; }"}})

	if !hasNotice(res, "part def Z, part def B no longer declared") {
		t.Errorf("notices = %v, want the typed declarations the load replaced", res.Notices)
	}
}

func TestOneLineBodyKeepsItsBraceWhenTheAdditionIsCommented(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	s.Submit("package P {\n\tpart def B; // add B\n}")

	res := s.Submit("part def Z;")
	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	if strings.Contains(out, "error") {
		t.Errorf("buffer stopped parsing:\n%s\n---\n%s", out, s.joined())
	}
}

func TestEverySiblingDeclarationIsConfirmedWhenOneSourceMerges(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")

	res := s.SubmitAll([]string{"package P { part def B; }", "part def X; part def Y;"})
	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "part def X", "part def Y")
}

// A comment typed above a body's first member documents the addition, so it
// comes along instead of disappearing into the merge.
func TestMergeKeepsACommentTypedAtTheTopOfTheBody(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	s.Submit("package P {\n\t// the engine of the car\n\tpart def Engine;\n}")

	if !strings.Contains(s.joined(), "// the engine of the car") {
		t.Errorf("buffer = %q, want the comment typed in the body", s.joined())
	}
	s.Submit("package P {\n\t// the engine of the car\n\tpart def Engine;\n}")
	if n := strings.Count(s.joined(), "// the engine of the car"); n != 1 {
		t.Errorf("comment present %d times in %q, want once", n, s.joined())
	}
}
