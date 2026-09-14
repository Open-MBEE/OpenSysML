package lsp

import (
	"context"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// refuseRename runs Rename and returns the refusal, failing when it succeeds.
func refuseRename(t *testing.T, ws *model.Workspace, name, cursorAt, newName string) string {
	t.Helper()
	got, err := applyRename(t, ws, name, cursorAt, newName)
	if err == nil {
		t.Fatalf("rename at %q to %q succeeded:\n%v", cursorAt, newName, got)
	}
	return err.Error()
}

// wantRefusal fails unless the refusal names every want.
func wantRefusal(t *testing.T, msg string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(msg, w) {
			t.Errorf("refusal does not say %q: %s", w, msg)
		}
	}
}

func TestRenameRefusesSiblingLongName(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/collide_long.sysml",
		"package P {\n\tpart def A;\n\tpart def B;\n\tpart a : A;\n}\n")
	msg := refuseRename(t, ws, name, "A;", "B")
	wantRefusal(t, msg, `P::A cannot be renamed to "B"`, "already means P::B")
}

func TestRenameRefusesSiblingShortName(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/collide_short.sysml",
		"package P {\n\tpart def A;\n\tpart def <b> Bee;\n\tpart a : A;\n}\n")
	msg := refuseRename(t, ws, name, "A;", "b")
	wantRefusal(t, msg, `P::A cannot be renamed to "b"`, "already means P::Bee")
}

func TestRenameRefusesShortNameOntoTakenName(t *testing.T) {
	const src = "package P {\n\tpart def <a> Alpha;\n\tpart def <b> Beta;\n\tpart def Gamma;\n\tpart x : a;\n}\n"
	for _, tt := range []struct{ newName, means string }{
		{"Gamma", "P::Gamma"},
		{"b", "P::Beta"},
		{"Beta", "P::Beta"},
	} {
		ws := model.NewWorkspace()
		name := openRenameDoc(t, ws, "/tmp/collide_short_decl.sysml", src)
		msg := refuseRename(t, ws, name, "a> Alpha", tt.newName)
		wantRefusal(t, msg, `P::Alpha cannot be renamed to "`+tt.newName+`"`, "already means "+tt.means)
	}
}

func TestRenameRefusesCaptureByInterveningDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/capture_nested.sysml",
		"package P {\n\tpart def Old;\n\tpart def Q {\n\t\tpart def New;\n\t\tpart u : Old;\n\t}\n}\n")
	msg := refuseRename(t, ws, name, "Old;", "New")
	wantRefusal(t, msg, `P::Old cannot be renamed to "New"`, "reference to it in P::Q", "would read P::Q::New instead")
}

func TestRenameRefusesQualifiedSegmentCollision(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/capture_qualified.sysml",
		"package A {\n\tpart def x;\n\tpart def y;\n}\npackage Q {\n\tpart p : A::x;\n}\n")
	msg := refuseRename(t, ws, name, "x;\n}", "y")
	wantRefusal(t, msg, `A::x cannot be renamed to "y"`, "already means A::y")
}

func TestRenameRefusesQualifiedCaptureThroughImport(t *testing.T) {
	// The owner declares no `y`, but the reference reaches x through a namespace
	// that does, so the rewritten segment `Q::y` would read it.
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/capture_qualified_member.sysml",
		"package P {\n\tpart def x;\n}\npackage Q {\n\timport P::*;\n\tpart def y;\n}\n"+
			"package R {\n\tpart p : Q::x;\n}\n")
	msg := refuseRename(t, ws, name, "x;", "y")
	wantRefusal(t, msg, `P::x cannot be renamed to "y"`, "reference to it in R", "would read Q::y instead")
}

// A qualifier respelled onto another element is captured even where that element
// lacks the member the rest of the name asks for: `B::x` reads B, then fails.
func TestRenameRefusesQualifierCaptureWhereTheSuffixIsMissing(t *testing.T) {
	for _, tt := range []struct{ name, visible, means string }{
		{"local", "\t\tpart def B;\n", "P::Q::B"},
		{"imported", "\t\timport Lib::B;\n", "Lib::B"},
	} {
		ws := model.NewWorkspace()
		name := openRenameDoc(t, ws, "/tmp/qualifier_"+tt.name+".sysml",
			"package Lib {\n\tpart def B;\n}\npackage P {\n\tpart def A { part def x; }\n\tpart def Q {\n"+
				tt.visible+"\t\tpart p : A::x;\n\t}\n}\n")
		msg := refuseRename(t, ws, name, "A {", "B")
		wantRefusal(t, msg, `P::A cannot be renamed to "B"`, "reference to it in P::Q", "would read "+tt.means+" instead")
	}
}

// A rewritten segment that would name several members leaves the reference
// ambiguous: a conflict, though the trial reading reaches no element.
func TestRenameRefusesSegmentLeftAmbiguous(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/ambiguous_segment.sysml",
		"package P {\n\tpart def x;\n}\npackage Q {\n\timport P::*;\n\tpart def y;\n\tattribute def y;\n}\n"+
			"package R {\n\tpart p : Q::x;\n}\n")
	msg := refuseRename(t, ws, name, "x;", "y")
	wantRefusal(t, msg, `P::x cannot be renamed to "y"`, "reference to it in R", "would name 2 elements at once")
}

// A renamed call selects by arguments among the overloads its new spelling denotes:
// another overload chosen, a tie, or an alias is refused; the target still chosen applies.
func TestRenameSelectsAmongOverloadsOfRenamedCall(t *testing.T) {
	const lib = "package Lib {\n\tcalc def g { in v : ScalarValues::Integer; return r : ScalarValues::Integer = v; }\n" +
		"\tcalc def h { in v : ScalarValues::Integer; return r : ScalarValues::Integer = v; }\n\talias gg for h;\n}\n"
	const realF = "package P {\n\tcalc def f { in v : ScalarValues::Real; return r : ScalarValues::Real = v; }\n}\n"
	const intF = "package P {\n\tcalc def f { in v : ScalarValues::Integer; return r : ScalarValues::Integer = v; }\n}\n"
	call := func(imports, arg string) string {
		return "package R {\n" + imports + "\tattribute c : ScalarValues::Real = f(" + arg + ");\n}\n"
	}
	const both = "\tprivate import P::*;\n\tprivate import Lib::*;\n"

	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/overload_other.sysml", lib+realF+call(both, "1"))
	msg := refuseRename(t, ws, name, "f {", "g")
	wantRefusal(t, msg, `P::f cannot be renamed to "g"`, "reference to it in R", "would read Lib::g instead")

	ws = model.NewWorkspace()
	name = openRenameDoc(t, ws, "/tmp/overload_tie.sysml", lib+intF+call(both, "1"))
	msg = refuseRename(t, ws, name, "f {", "g")
	wantRefusal(t, msg, `P::f cannot be renamed to "g"`, "reference to it in R", "would name 2 elements at once")

	ws = model.NewWorkspace()
	name = openRenameDoc(t, ws, "/tmp/overload_alias.sysml", lib+realF+call(both, "1"))
	msg = refuseRename(t, ws, name, "f {", "gg")
	wantRefusal(t, msg, `P::f cannot be renamed to "gg"`, "reference to it in R", "would read Lib::gg instead")

	// The name alone still reads the target through its short name; the arguments do not.
	ws = model.NewWorkspace()
	name = openRenameDoc(t, ws, "/tmp/overload_short.sysml", lib+strings.Replace(realF, "def f", "def <g> f", 1)+call(both, "1"))
	msg = refuseRename(t, ws, name, "f {", "g")
	wantRefusal(t, msg, `P::f cannot be renamed to "g"`, "reference to it in R", "would read Lib::g instead")

	for _, imports := range []string{both, "\tprivate import Lib::*;\n\tprivate import P::*;\n"} {
		ws = model.NewWorkspace()
		name = openRenameDoc(t, ws, "/tmp/overload_kept.sysml", lib+realF+call(imports, "1.5"))
		got, err := applyRename(t, ws, name, "f {", "g")
		if err != nil {
			t.Fatalf("Rename err = %v", err)
		}
		want := lib + strings.Replace(realF, "def f", "def g", 1) + strings.Replace(call(imports, "1.5"), "= f(", "= g(", 1)
		if got[name] != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
		}
	}
}

// The same for a feature chain's qualified member, which is read outward from
// the operand's type and whose failed reading the resolver otherwise discards.
func TestRenameRefusesQualifierCaptureInFeatureChainMember(t *testing.T) {
	const src = "package P {\n\tpart def A { part x; }\n\tpart def Q {\n\t\tpart def B;\n" +
		"\t\tpart d : P::A;\n\t\tpart e :> d.A::x;\n\t}\n}\n"
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/qualifier_chain.sysml", src)
	msg := refuseRename(t, ws, name, "A {", "B")
	wantRefusal(t, msg, `P::A cannot be renamed to "B"`, "reference to it in P::Q", "would read P::Q::B instead")

	ws = model.NewWorkspace()
	name = openRenameDoc(t, ws, "/tmp/qualifier_chain_clean.sysml", src)
	got, err := applyRename(t, ws, name, "A {", "C")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := "package P {\n\tpart def C { part x; }\n\tpart def Q {\n\t\tpart def B;\n" +
		"\t\tpart d : P::C;\n\t\tpart e :> d.C::x;\n\t}\n}\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

func TestRenameRefusesCaptureInAnotherDocument(t *testing.T) {
	ws := model.NewWorkspace()
	declName := openRenameDoc(t, ws, "/tmp/capture_decl.sysml", "package P {\n\tpart def Old;\n}\n")
	openRenameDoc(t, ws, "/tmp/capture_uses.sysml",
		"package Q {\n\timport P::*;\n\tpart def New;\n\tpart u : Old;\n}\n")
	msg := refuseRename(t, ws, declName, "Old;", "New")
	wantRefusal(t, msg, `P::Old cannot be renamed to "New"`, "reference to it in Q", "would read Q::New instead")
}

func TestRenameRefusesTakingAnAliasForItself(t *testing.T) {
	// The alias names the target itself, but it is a distinct membership: the
	// rename would leave `alias New for New`.
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/alias_self.sysml",
		"package P {\n\tpart def Old;\n\talias New for Old;\n\tpart x : Old;\n}\n")
	msg := refuseRename(t, ws, name, "Old;", "New")
	wantRefusal(t, msg, `P::Old cannot be renamed to "New"`, "already means P::New")
}

func TestRenameRefusesCaptureByAnAliasForItself(t *testing.T) {
	// The rewritten references would be read through the alias, which the rename
	// turns cyclic.
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/alias_capture.sysml",
		"package P {\n\tpart def Old;\n\tpart def Q {\n\t\talias New for Old;\n\t\tpart u : Old;\n\t}\n}\n")
	msg := refuseRename(t, ws, name, "Old;", "New")
	wantRefusal(t, msg, `P::Old cannot be renamed to "New"`, "reference to it in P::Q", "would read P::Q::New instead")
}

func TestRenameRefusesShadowingAnOuterName(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/shadow_outer.sysml",
		"package Demo {\n\tattribute x = 1;\n\tpart def P {\n\t\tattribute y = 2;\n\t\tattribute z = x;\n\t}\n}\n")
	msg := refuseRename(t, ws, name, "y = 2", "x")
	wantRefusal(t, msg, `Demo::P::y cannot be renamed to "x"`, "already means Demo::x")
}

// A feature chain's member is read in the operand's type, not where the chain is
// written: a subtype's own member captures the renamed inherited one there.
func TestRenameRefusesCaptureThroughFeatureChain(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/capture_chain.sysml",
		"package P {\n\tpart def Base { part x; }\n\tpart def Derived :> Base { part y; }\n"+
			"\tpart d : Derived;\n\tpart e :> d.x;\n}\n")
	msg := refuseRename(t, ws, name, "x; }", "y")
	wantRefusal(t, msg, `P::Base::x cannot be renamed to "y"`, "reference to it in P", "would read P::Derived::y instead")
}

func TestRenameSucceedsThroughFeatureChainWhenSubtypeLacksName(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/chain_clean.sysml",
		"package P {\n\tpart def Base { part x; }\n\tpart def Derived :> Base { part y; }\n"+
			"\tpart d : Derived;\n\tpart e :> d.x;\n}\n")
	got, err := applyRename(t, ws, name, "x; }", "z")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := "package P {\n\tpart def Base { part z; }\n\tpart def Derived :> Base { part y; }\n" +
		"\tpart d : Derived;\n\tpart e :> d.z;\n}\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

// A chained transition end's member is read in the vertex its operand names: a
// state the usage declares itself captures the renamed inherited one there.
func TestRenameRefusesCaptureThroughChainedTransitionEnd(t *testing.T) {
	const src = "package P {\n\tstate def O { state old; }\n\tstate def M {\n\t\tentry; then idle;\n\t\tstate idle;\n" +
		"\t\tstate outer : O { state taken; }\n\t\tfirst idle then outer.old;\n\t}\n}\n"
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/capture_chain_end.sysml", src)
	msg := refuseRename(t, ws, name, "old;", "taken")
	wantRefusal(t, msg, `P::O::old cannot be renamed to "taken"`, "reference to it in P::M", "would read P::M::outer::taken instead")

	ws = model.NewWorkspace()
	name = openRenameDoc(t, ws, "/tmp/capture_chain_end_clean.sysml", src)
	got, err := applyRename(t, ws, name, "old;", "fresh")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := "package P {\n\tstate def O { state fresh; }\n\tstate def M {\n\t\tentry; then idle;\n\t\tstate idle;\n" +
		"\t\tstate outer : O { state taken; }\n\t\tfirst idle then outer.fresh;\n\t}\n}\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

// Renaming a name to itself changes nothing, even where the declaration shadows
// an outer namesake that the collision check would otherwise report.
func TestRenameToTheSameNameIsNotAConflict(t *testing.T) {
	ws := model.NewWorkspace()
	const src = "package Demo {\n\tattribute x = 1;\n\tpart def P {\n\t\tattribute x = 2;\n\t\tattribute z = x;\n\t}\n}\n"
	name := openRenameDoc(t, ws, "/tmp/same_name.sysml", src)
	got, err := applyRename(t, ws, name, "x = 2", "x")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if got[name] != src {
		t.Fatalf("got:\n%s\nwant the document unchanged", got[name])
	}
}

func TestRenameSucceedsWhenNameTakenOnlyInUnrelatedScope(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/unrelated_scope.sysml",
		"package P {\n\tpart def Old;\n\tpart u : Old;\n}\npackage Other {\n\tpart def New;\n}\n")
	got, err := applyRename(t, ws, name, "Old;", "New")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := "package P {\n\tpart def New;\n\tpart u : New;\n}\npackage Other {\n\tpart def New;\n}\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

func TestRenameSucceedsWhenOnlySameNameIsTheTargetItself(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/self_redefinition.sysml",
		"part def Base { part x; }\npart def Derived :> Base { part redefines x; }\n")
	got, err := applyRename(t, ws, name, "x; }", "y")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := "part def Base { part y; }\npart def Derived :> Base { part redefines y; }\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

// PrepareRename cannot know the new name, so a name whose every rename would be
// refused is still offered for editing; the refusal comes from Rename.
func TestPrepareRenameOffersNameWhoseRenameMayCollide(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/prepare_collide.sysml", "package P {\n\tpart def A;\n\tpart def B;\n}\n")
	s := NewServer(ws)
	doc := ws.Document(name)
	off := strings.Index(string(doc.Content), "A;")
	rng, err := s.PrepareRename(context.Background(), &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(doc.Content, off),
		},
	})
	if err != nil || rng == nil {
		t.Fatalf("PrepareRename = %v, %v", rng, err)
	}
	if rng.Start.Line != 1 || rng.Start.Character != 10 || rng.End.Character != 11 {
		t.Fatalf("PrepareRename range = %+v, want the `A` identifier", *rng)
	}
}
