package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// ambiguousNavigationSrc imports two equally specific `pick(Integer)` overloads and a
// broader `pick(Real)`, so `pick(2)` is tied between the two while `pick("s")` selects
// the String one.
const ambiguousNavigationSrc = `package A {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = 1; }
}
package B {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = 2; }
	calc def pick { in x : String; return : Integer = 3; }
}
package C {
	private import ScalarValues::*;
	calc def pick { in x : Real; return : Integer = 4; }
}
package P {
	private import ScalarValues::*;
	private import A::*;
	private import B::*;
	private import C::*;
	attribute i : Integer = pick(2);
	attribute s : Integer = pick("s");
}
`

func openAmbiguousNavigation(t *testing.T) (*model.Workspace, *Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	name := uri.File("/tmp/ambiguous_nav.sysml").Filename()
	ws.Open(name, []byte(ambiguousNavigationSrc), 1)
	return ws, NewServer(ws), name
}

// lineOfSrc is the zero-based line the first occurrence of needle is on.
func lineOfSrc(t *testing.T, src, needle string) uint32 {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not in source", needle)
	}
	return uint32(strings.Count(src[:off], "\n"))
}

func referencesAt(t *testing.T, s *Server, name, src, needle string) []protocol.Location {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not in source", needle)
	}
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	return locs
}

// Go-to-definition on an ambiguous call lists every overload the arguments leave
// tied, in declaration order, and not the broader one they also fit; a call that
// selects one still goes to it.
func TestDefinitionListsAmbiguousOverloads(t *testing.T) {
	_, s, name := openAmbiguousNavigation(t)
	src := ambiguousNavigationSrc

	locs := definitionOf(t, s, name, src, "pick(2)")
	if len(locs) != 2 {
		t.Fatalf("pick(2): locations = %d, want only the two Integer overloads: %v", len(locs), locs)
	}
	wantLines := []uint32{
		lineOfSrc(t, src, "calc def pick { in x : Integer; return : Integer = 1;"),
		lineOfSrc(t, src, "calc def pick { in x : Integer; return : Integer = 2;"),
	}
	for i, loc := range locs {
		if loc.URI != uri.File(name) {
			t.Errorf("location %d URI = %q, want %q", i, loc.URI, uri.File(name))
		}
		if loc.Range.Start.Line != wantLines[i] {
			t.Errorf("location %d line = %d, want %d", i, loc.Range.Start.Line, wantLines[i])
		}
	}

	locs = definitionOf(t, s, name, src, `pick("s")`)
	if len(locs) != 1 {
		t.Fatalf(`pick("s"): locations = %d, want 1`, len(locs))
	}
	if got, want := locs[0].Range.Start.Line, lineOfSrc(t, src, "in x : String"); got != want {
		t.Errorf(`pick("s"): line = %d, want %d`, got, want)
	}
}

// Find-references never counts an ambiguous call for any of its overloads, and
// from the ambiguous call itself there is no one declaration to list uses of.
func TestReferencesSkipAmbiguousCalls(t *testing.T) {
	_, s, name := openAmbiguousNavigation(t)
	src := ambiguousNavigationSrc

	for _, decl := range []string{
		"pick { in x : Integer; return : Integer = 1;",
		"pick { in x : Integer; return : Integer = 2;",
	} {
		if locs := referencesAt(t, s, name, src, decl); len(locs) != 0 {
			t.Errorf("%s: references = %v, want none (the only call is ambiguous)", decl, locs)
		}
	}
	if locs := referencesAt(t, s, name, src, "pick { in x : Real"); len(locs) != 0 {
		t.Errorf("Real overload: references = %v, want none (it is beaten, not tied)", locs)
	}
	locs := referencesAt(t, s, name, src, "pick { in x : String")
	if len(locs) != 1 {
		t.Fatalf("String overload: references = %d, want the one selecting call: %v", len(locs), locs)
	}
	if at := positionToOffset([]byte(src), locs[0].Range.Start); !strings.HasPrefix(src[at:], `pick("s")`) {
		t.Errorf("reference at %q, want the String call", src[at:at+9])
	}
	if locs := referencesAt(t, s, name, src, "pick(2)"); len(locs) != 0 {
		t.Errorf("from the ambiguous call: references = %v, want none", locs)
	}
}

// Rename leaves an ambiguous call as written, whichever tied overload is renamed,
// and refuses to start from the call itself.
func TestRenameSkipsAmbiguousCalls(t *testing.T) {
	ws, _, name := openAmbiguousNavigation(t)

	out, err := applyRename(t, ws, name, "pick { in x : Integer; return : Integer = 1;", "choose")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	got := out[name]
	for _, want := range []string{
		"calc def choose { in x : Integer; return : Integer = 1;",
		"calc def pick { in x : Integer; return : Integer = 2;",
		"= pick(2);",
		`= pick("s");`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renamed source lacks %q:\n%s", want, got)
		}
	}

	out, err = applyRename(t, ws, name, "pick { in x : String", "text")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	got = out[name]
	for _, want := range []string{"calc def text { in x : String", `= text("s");`, "= pick(2);"} {
		if !strings.Contains(got, want) {
			t.Errorf("renamed source lacks %q:\n%s", want, got)
		}
	}

	if _, err := applyRename(t, ws, name, "pick(2)", "choose"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("rename from the ambiguous call: err = %v, want an ambiguity error", err)
	}
}

// Hover on an ambiguous call names each tied overload rather than one of them
// or the declaration the call sits in.
func TestHoverNamesAmbiguousOverloads(t *testing.T) {
	_, s, name := openAmbiguousNavigation(t)
	src := ambiguousNavigationSrc
	off := strings.Index(src, "pick(2)")
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil {
		t.Fatal("Hover = nil, want the tied overloads")
	}
	for _, want := range []string{"A::pick", "B::pick", "Ambiguous call"} {
		if !strings.Contains(hov.Contents.Value, want) {
			t.Errorf("hover %q lacks %q", hov.Contents.Value, want)
		}
	}
	if strings.Contains(hov.Contents.Value, "C::pick") {
		t.Errorf("hover %q names the beaten Real overload", hov.Contents.Value)
	}
	if strings.Contains(hov.Contents.Value, "attribute") {
		t.Errorf("hover %q describes the enclosing attribute", hov.Contents.Value)
	}
	if hov.Range == nil || hov.Range.Start.Line != lineOfSrc(t, src, "pick(2)") {
		t.Errorf("hover range = %v, want the call's name", hov.Range)
	}
}

// The ambiguous-call hover ranges over the content its reference was collected
// from, so a document removed between the two steps of one request cannot crash it.
func TestAmbiguousHoverSurvivesDocumentRemoval(t *testing.T) {
	ws, s, name := openAmbiguousNavigation(t)
	doc := ws.Document(name)
	content := doc.Content
	off := strings.Index(string(content), "pick(2)")
	ref := refAtOffset(collectRefs(doc.AST, doc.Scope), off)
	if ref == nil {
		t.Fatal("no reference at pick(2)")
	}
	ws.Remove(name)
	if s.document(name) != nil {
		t.Fatal("document still present after removal")
	}
	hov := s.ambiguousCallHover(name, content, *ref, off)
	if hov == nil {
		t.Fatal("ambiguousCallHover = nil, want the tied overloads")
	}
	if hov.Range == nil || hov.Range.Start.Line != lineOfSrc(t, string(content), "pick(2)") {
		t.Errorf("hover range = %v, want the call's name", hov.Range)
	}
}

// qualifiedAmbiguitySrc reaches two equally specific `pick(Integer)` overloads through
// the qualifier `Both` and through two aliases named `choose`, so `Both::pick(2)` and
// `choose(2)` are tied while `Both::pick("s")` selects the String one.
const qualifiedAmbiguitySrc = `package A {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = 1; }
}
package B {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = 2; }
}
package S {
	private import ScalarValues::*;
	calc def pick { in x : String; return : Integer = 3; }
}
package Both {
	public import A::*;
	public import B::*;
	public import S::*;
}
package AL { alias choose for A::pick; }
package BL { alias choose for B::pick; }
package Q {
	private import ScalarValues::*;
	private import AL::*;
	private import BL::*;
	attribute qi : Integer = Both::pick(2);
	attribute qs : Integer = Both::pick("s");
	attribute ci : Integer = choose(2);
}
`

func openQualifiedAmbiguity(t *testing.T) (*model.Workspace, *Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	name := uri.File("/tmp/qualified_ambiguity.sysml").Filename()
	ws.Open(name, []byte(qualifiedAmbiguitySrc), 1)
	return ws, NewServer(ws), name
}

// startLines are the zero-based lines the locations start on, in order.
func startLines(locs []protocol.Location) []uint32 {
	out := make([]uint32, len(locs))
	for i, loc := range locs {
		out[i] = loc.Range.Start.Line
	}
	return out
}

// On a qualified ambiguous call, only the called name lists the tied overloads;
// the qualifier goes to the namespace it names as anywhere else.
func TestDefinitionOnQualifiedAmbiguousCall(t *testing.T) {
	_, s, name := openQualifiedAmbiguity(t)
	src := qualifiedAmbiguitySrc

	locs := definitionOf(t, s, name, src, "Both::pick(2)")
	if want := []uint32{lineOfSrc(t, src, "package Both")}; !equalLines(startLines(locs), want) {
		t.Errorf("qualifier Both: lines = %v, want %v", startLines(locs), want)
	}

	locs = definitionOf(t, s, name, src, "pick(2)")
	want := []uint32{
		lineOfSrc(t, src, "in x : Integer; return : Integer = 1;"),
		lineOfSrc(t, src, "in x : Integer; return : Integer = 2;"),
	}
	if !equalLines(startLines(locs), want) {
		t.Errorf("Both::pick(2): lines = %v, want the two tied overloads %v", startLines(locs), want)
	}

	locs = definitionOf(t, s, name, src, `pick("s")`)
	if want := []uint32{lineOfSrc(t, src, "in x : String")}; !equalLines(startLines(locs), want) {
		t.Errorf(`Both::pick("s"): lines = %v, want %v`, startLines(locs), want)
	}
}

// The qualifier of an ambiguous call is still a reference to its namespace, both
// when listing the namespace's references and when starting from the qualifier;
// only the called name is left out.
func TestReferencesOnQualifiedAmbiguousCall(t *testing.T) {
	_, s, name := openQualifiedAmbiguity(t)
	src := qualifiedAmbiguitySrc

	wantBoth := []uint32{lineOfSrc(t, src, "Both::pick(2)"), lineOfSrc(t, src, `Both::pick("s")`)}
	for _, at := range []string{"Both {", "Both::pick(2)"} {
		locs := referencesAt(t, s, name, src, at)
		if !equalLines(startLines(locs), wantBoth) {
			t.Errorf("from %q: reference lines = %v, want both qualified calls %v", at, startLines(locs), wantBoth)
		}
	}
	if locs := referencesAt(t, s, name, src, "pick(2)"); len(locs) != 0 {
		t.Errorf("from the tied called name: references = %v, want none", locs)
	}
	for decl, alias := range map[string]string{
		"pick { in x : Integer; return : Integer = 1;": "alias choose for A::pick",
		"pick { in x : Integer; return : Integer = 2;": "alias choose for B::pick",
	} {
		locs := referencesAt(t, s, name, src, decl)
		if want := []uint32{lineOfSrc(t, src, alias)}; !equalLines(startLines(locs), want) {
			t.Errorf("%s: reference lines = %v, want only its alias %v (every call to it is tied)", decl, startLines(locs), want)
		}
	}
	locs := referencesAt(t, s, name, src, "pick { in x : String")
	if want := []uint32{lineOfSrc(t, src, `Both::pick("s")`)}; !equalLines(startLines(locs), want) {
		t.Errorf("String overload: reference lines = %v, want %v", startLines(locs), want)
	}
}

// A call written through an alias is tied when the aliases in scope name equally
// specific overloads: rename refuses to start from it, renaming either alias leaves
// it as written, and neither alias counts it among its references.
func TestRenameSkipsAliasedAmbiguousCall(t *testing.T) {
	ws, s, name := openQualifiedAmbiguity(t)
	src := qualifiedAmbiguitySrc

	if _, err := applyRename(t, ws, name, "choose(2)", "select"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("rename from the aliased ambiguous call: err = %v, want an ambiguity error", err)
	}
	for _, alias := range []string{"choose for A::pick", "choose for B::pick"} {
		out, err := applyRename(t, ws, name, alias, "select")
		if err != nil {
			t.Fatalf("rename %q: err = %v", alias, err)
		}
		got := out[name]
		if !strings.Contains(got, "alias select for "+strings.TrimPrefix(alias, "choose for ")) {
			t.Errorf("rename %q: alias not renamed:\n%s", alias, got)
		}
		if !strings.Contains(got, "= choose(2);") {
			t.Errorf("rename %q: the tied call was rewritten:\n%s", alias, got)
		}
		if locs := referencesAt(t, s, name, src, alias); len(locs) != 0 {
			t.Errorf("alias %q: references = %v, want none (its only call is tied)", alias, locs)
		}
	}
}

// selectingAliasesSrc has two aliases named `conv`, for an Integer and a String overload,
// so each call selects one alias's target and is a use of that alias only.
const selectingAliasesSrc = `package A {
	private import ScalarValues::*;
	calc def toText { in x : Integer; return : String = "i"; }
}
package B {
	private import ScalarValues::*;
	calc def toText { in x : String; return : String = x; }
}
package AL { alias conv for A::toText; }
package BL { alias conv for B::toText; }
package Names {
	public import AL::*;
	public import BL::*;
}
package Q {
	private import ScalarValues::*;
	private import AL::*;
	private import BL::*;
	attribute i : String = conv(2);
	attribute s : String = conv("s");
	attribute qi : String = Names::conv(3);
	attribute qs : String = Names::conv("t");
}
`

// A call through one of two same-named aliases is a use of the alias whose target it
// selects, not the first found: references list it there only; renaming the other skips it.
func TestReferencesAndRenameFollowSelectedAlias(t *testing.T) {
	src := selectingAliasesSrc
	ws := model.NewWorkspace()
	name := uri.File("/tmp/selecting_aliases.sysml").Filename()
	ws.Open(name, []byte(src), 1)
	s := NewServer(ws)

	for alias, calls := range map[string][]string{
		"conv for A::toText": {"conv(2)", "Names::conv(3)"},
		"conv for B::toText": {`conv("s")`, `Names::conv("t")`},
	} {
		want := make([]uint32, 0, len(calls))
		for _, call := range calls {
			want = append(want, lineOfSrc(t, src, call))
		}
		if locs := referencesAt(t, s, name, src, alias); !equalLines(startLines(locs), want) {
			t.Errorf("alias %q: reference lines = %v, want the calls selecting its target %v", alias, startLines(locs), want)
		}
	}

	out, err := applyRename(t, ws, name, "conv for B::toText", "text")
	if err != nil {
		t.Fatalf("rename the String alias: err = %v", err)
	}
	got := out[name]
	for _, want := range []string{
		"alias conv for A::toText", "alias text for B::toText",
		"= conv(2);", `= text("s");`, "= Names::conv(3);", `= Names::text("t");`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renamed source lacks %q:\n%s", want, got)
		}
	}

	locs := definitionOf(t, s, name, src, `conv("s")`)
	if want := []uint32{lineOfSrc(t, src, "in x : String")}; !equalLines(startLines(locs), want) {
		t.Errorf(`definition of conv("s"): lines = %v, want the String overload %v`, startLines(locs), want)
	}
}

func equalLines(got, want []uint32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// hoverOf hovers the first occurrence of needle.
func hoverOf(t *testing.T, s *Server, name, src, needle string) *protocol.Hover {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not in source", needle)
	}
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	return hov
}

// A cursor on the `::` of a qualified call is on no segment: hover describes the
// enclosing declaration, as for any qualified name, while definition and references
// treat the separator as the whole reference — the tied set for an ambiguous call,
// the selected overload otherwise.
func TestSeparatorOfQualifiedAmbiguousCall(t *testing.T) {
	_, s, name := openQualifiedAmbiguity(t)
	src := qualifiedAmbiguitySrc

	for _, at := range []string{"::pick(2)", `::pick("s")`} {
		hov := hoverOf(t, s, name, src, at)
		if hov == nil || !strings.Contains(hov.Contents.Value, "attribute q") || strings.Contains(hov.Contents.Value, "Ambiguous") {
			t.Errorf("hover on %q = %v, want the enclosing attribute", at, hov)
		}
	}

	locs := definitionOf(t, s, name, src, "::pick(2)")
	want := []uint32{
		lineOfSrc(t, src, "in x : Integer; return : Integer = 1;"),
		lineOfSrc(t, src, "in x : Integer; return : Integer = 2;"),
	}
	if !equalLines(startLines(locs), want) {
		t.Errorf("definition on `::` of Both::pick(2): lines = %v, want the tied overloads %v", startLines(locs), want)
	}
	if locs := referencesAt(t, s, name, src, "::pick(2)"); len(locs) != 0 {
		t.Errorf("references on `::` of Both::pick(2) = %v, want none", locs)
	}

	locs = definitionOf(t, s, name, src, `::pick("s")`)
	if want := []uint32{lineOfSrc(t, src, "in x : String")}; !equalLines(startLines(locs), want) {
		t.Errorf(`definition on "::" of Both::pick("s"): lines = %v, want %v`, startLines(locs), want)
	}
}

// A `$::` with no name after it — mid-edit — is a reference with no segments;
// references and rename scan every reference and must pass over it.
func TestReferencesAndRenameSkipNamelessReference(t *testing.T) {
	src := `package P {
	part def Wheel;
	part w : Wheel;
	part x : $::;
}
`
	ws := model.NewWorkspace()
	name := uri.File("/tmp/nameless_ref.sysml").Filename()
	ws.Open(name, []byte(src), 1)
	s := NewServer(ws)

	locs := referencesAt(t, s, name, src, "Wheel;")
	if want := []uint32{lineOfSrc(t, src, "w : Wheel")}; !equalLines(startLines(locs), want) {
		t.Errorf("references of Wheel: lines = %v, want %v", startLines(locs), want)
	}

	out, err := applyRename(t, ws, name, "Wheel;", "Tyre")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	got := out[name]
	for _, want := range []string{"part def Tyre;", "part w : Tyre;", "part x : $::;"} {
		if !strings.Contains(got, want) {
			t.Errorf("renamed source lacks %q:\n%s", want, got)
		}
	}
}
