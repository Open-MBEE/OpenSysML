package edit

import (
	"strings"
	"testing"
)

// renamed applies one rename to src and returns the edited notation, checking
// that the result is as valid as the original was.
func renamed(t *testing.T, name, src, target, newName string) string {
	t.Helper()
	m := loadContent(t, name, src)
	requireClean(t, m)
	res := applyOne(t, m, Rename(target, newName))
	assertOnlySpanChanged(t, m, res)
	requireClean(t, loadContent(t, name, string(res.Content)))
	return string(res.Content)
}

// refusedRename applies one rename expected to be refused and returns the error.
func refusedRename(t *testing.T, name, src, target, newName string) *Error {
	t.Helper()
	m := loadContent(t, name, src)
	requireClean(t, m)
	res, err := Apply(m, []Operation{Rename(target, newName)})
	if res != nil {
		t.Fatalf("refused rename returned content:\n%s", res.Content)
	}
	return editError(t, err)
}

func TestRenameRewritesQualifiedReferences(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tpart def Old;\n" +
		"\tpart a : P::Old;\n\tpart b : Old;\n}\n"
	got := renamed(t, "qualified.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{"part a : P::Fresh;", "part b : Old;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	// Q's own Old is a different element, so neither its declaration nor the
	// reference to it moves.
	if !strings.Contains(got, "package Q {\n\tpart def Old;") {
		t.Fatalf("an unrelated declaration was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "package P {\n\tpart def Fresh;") {
		t.Fatalf("the declaration was not renamed:\n%s", got)
	}
}

func TestRenameRewritesImportedReferences(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::Old;\n" +
		"\tpart a : Old;\n\tpart b : P::Old;\n}\n"
	got := renamed(t, "imported.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{
		"part def Fresh;", "import P::Fresh;", "part a : Fresh;", "part b : P::Fresh;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Old") {
		t.Fatalf("the old name survives:\n%s", got)
	}
}

// A wildcard import names the namespace, not the member, so renaming a member
// leaves the import alone.
func TestRenameLeavesWildcardImportAlone(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::*;\n" +
		"\tpart a : Old;\n}\n"
	got := renamed(t, "wildcard.sysml", src, "P::Old", "Fresh")

	if !strings.Contains(got, "import P::*;") || !strings.Contains(got, "part a : Fresh;") {
		t.Fatalf("wildcard import rename is wrong:\n%s", got)
	}
}

// An alias is a reference: renaming the element rewrites the alias target, and
// the alias's own name — what references read — does not change.
func TestRenameRewritesAliasTargetNotAliasUses(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\talias A for Old;\n\tpart a : A;\n" +
		"\tpart b : Old;\n}\n"
	got := renamed(t, "alias.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{
		"part def Fresh;", "alias A for Fresh;", "part a : A;", "part b : Fresh;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// Renaming the alias itself rewrites what reads the alias, and not the element
// the alias points at.
func TestRenameAliasRewritesItsUses(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\talias A for Old;\n\tpart a : A;\n}\n"
	got := renamed(t, "alias-rename.sysml", src, "P::A", "B")

	for _, want := range []string{"part def Old;", "alias B for Old;", "part a : B;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// A reference written with the element's short name still resolves after the
// long name is renamed, so it is left as written.
func TestRenameLeavesShortNameReferencesAlone(t *testing.T) {
	const src = "package P {\n\tpart def <O> Old;\n\tpart a : O;\n\tpart b : Old;\n}\n"
	got := renamed(t, "shortname.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{"part def <O> Fresh;", "part a : O;", "part b : Fresh;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// The rename is refused where the new name already means something else at a
// reference site: rewriting there would silently rebind that reference, which
// re-analysis cannot catch because the name still resolves.
func TestRenameCapturingANameAtAReferenceIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::*;\n" +
		"\tpart def New;\n\tpart a : Old;\n}\n"
	e := refusedRename(t, "capture.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "Q::New") {
		t.Fatalf("refusal does not name what the reference would read: %s", e.Message)
	}
	if len(e.Referring) != 1 || e.Referring[0] != "Q" {
		t.Fatalf("refusal reports referring %v, want [Q]", e.Referring)
	}
}

// The same refusal for a feature chain's member, which is read in the operand's
// type rather than where the chain is written: the subtype's own member captures it.
func TestRenameCapturingAFeatureChainMemberIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Base { part x; }\n\tpart def Derived :> Base { part y; }\n" +
		"\tpart d : Derived;\n\tpart e :> d.x;\n}\n"
	e := refusedRename(t, "capture-chain.sysml", src, "P::Base::x", "y")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "P::Derived::y") {
		t.Fatalf("refusal does not name what the chain would read: %s", e.Message)
	}
	if len(e.Referring) != 1 || e.Referring[0] != "P" {
		t.Fatalf("refusal reports referring %v, want [P]", e.Referring)
	}
}

// The same refusal where the new name is an alias for the element itself: the
// references would be read through the alias, which the rename turns cyclic.
func TestRenameCapturingByAnAliasForItselfIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\tpart def Q {\n\t\talias New for Old;\n" +
		"\t\tpart u : Old;\n\t}\n}\n"
	e := refusedRename(t, "capture-alias.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "P::Q::New") {
		t.Fatalf("refusal does not name the alias: %s", e.Message)
	}
	if len(e.Referring) != 1 || e.Referring[0] != "P::Q" {
		t.Fatalf("refusal reports referring %v, want [P::Q]", e.Referring)
	}
}

// The same refusal for a qualified reference: the new name is already a member
// of the namespace the reference qualifies through.
func TestRenameCapturingAQualifiedSegmentIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\tpart def New;\n}\npackage Q {\n" +
		"\tpart a : P::Old;\n}\n"
	e := refusedRename(t, "capture-qualified.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "P::New") {
		t.Fatalf("refusal does not name P::New: %s", e.Message)
	}
}

// A qualifier respelled onto a visible element is captured by it even where that
// element lacks the rest of the name, as a feature chain's outward-read member is.
func TestRenameCapturingAQualifierWithoutTheSuffixIsRefused(t *testing.T) {
	for _, tt := range []struct{ name, q, use string }{
		{"subsetted", "Q :> P::Old", "\t\tpart y :> Old::x;\n"},
		{"chain", "Q", "\t\tpart d : P::Old;\n\t\tpart e :> d.Old::x;\n"},
	} {
		src := "package P {\n\tpart def Old { part x; }\n\tpart def " + tt.q + " {\n\t\tpart def New;\n" + tt.use + "\t}\n}\n"
		e := refusedRename(t, "capture-qualifier-"+tt.name+".sysml", src, "P::Old", "New")

		if e.Failure != FailureInvalidName {
			t.Fatalf("%s: failure is %s (%s), want invalid-name", tt.name, e.Failure, e.Message)
		}
		if !strings.Contains(e.Message, "P::Q::New") {
			t.Fatalf("%s: refusal does not name P::Q::New: %s", tt.name, e.Message)
		}
		if len(e.Referring) != 1 || e.Referring[0] != "P::Q" {
			t.Fatalf("%s: refusal reports referring %v, want [P::Q]", tt.name, e.Referring)
		}
	}
}

// A redefinition's target is read from the owning type's generals and then its
// enclosing namespace, never the owning type's own members, so a sibling of the
// redefining feature cannot capture the respelled qualifier.
func TestRenameQualifierOfARedefinitionIsNotCapturedByASibling(t *testing.T) {
	const src = "package P {\n\tpart def Old { part x; }\n\tpart def Q :> P::Old {\n\t\tpart def New;\n" +
		"\t\tpart y :>> Old::x;\n\t}\n}\n"
	got := renamed(t, "redefinition-qualifier.sysml", src, "P::Old", "New")

	for _, want := range []string{"part def New { part x; }", "part def Q :> P::New {", "part y :>> New::x;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// A rewritten segment that would name several members leaves the reference
// ambiguous, which is refused too. Only a namespace with duplicate names makes a
// segment ambiguous, so the fixture is deliberately ill-formed there.
func TestRenameLeavingAQualifiedSegmentAmbiguousIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\timport P::*;\n\tpart def New;\n" +
		"\tattribute def New;\n}\npackage R {\n\tpart p : Q::Old;\n}\n"
	m := loadContent(t, "ambiguous-segment.sysml", src)
	res, err := Apply(m, []Operation{Rename("P::Old", "New")})
	if res != nil {
		t.Fatalf("refused rename returned content:\n%s", res.Content)
	}
	e := editError(t, err)

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "would name 2 elements at once") {
		t.Fatalf("refusal does not report the ambiguity: %s", e.Message)
	}
	if len(e.Referring) != 1 || e.Referring[0] != "R" {
		t.Fatalf("refusal reports referring %v, want [R]", e.Referring)
	}
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

	for _, tt := range []struct{ name, src, newName, want string }{
		{"other", lib + realF + call(both, "1"), "g", "would read Lib::g instead"},
		{"tie", lib + intF + call(both, "1"), "g", "would name 2 elements at once"},
		{"alias", lib + realF + call(both, "1"), "gg", "would read Lib::gg instead"},
		// The name alone still reads the target through its short name; the arguments do not.
		{"short", lib + strings.Replace(realF, "def f", "def <g> f", 1) + call(both, "1"), "g", "would read Lib::g instead"},
	} {
		e := refusedRename(t, "overload-"+tt.name+".sysml", tt.src, "P::f", tt.newName)
		if e.Failure != FailureInvalidName {
			t.Fatalf("%s: failure is %s (%s), want invalid-name", tt.name, e.Failure, e.Message)
		}
		if !strings.Contains(e.Message, tt.want) {
			t.Fatalf("%s: refusal does not say %q: %s", tt.name, tt.want, e.Message)
		}
		if len(e.Referring) != 1 || e.Referring[0] != "R" {
			t.Fatalf("%s: refusal reports referring %v, want [R]", tt.name, e.Referring)
		}
	}

	for _, imports := range []string{both, "\tprivate import Lib::*;\n\tprivate import P::*;\n"} {
		m := loadContent(t, "overload-kept.sysml", lib+realF+call(imports, "1.5"))
		requireClean(t, m)
		res, err := Apply(m, []Operation{Rename("P::f", "g")})
		if err != nil {
			t.Fatalf("rename refused: %v", err)
		}
		want := lib + strings.Replace(realF, "def f", "def g", 1) + strings.Replace(call(imports, "1.5"), "= f(", "= g(", 1)
		if string(res.Content) != want {
			t.Fatalf("got:\n%s\nwant:\n%s", res.Content, want)
		}
	}
}

// Renaming onto a name declared in a scope the element's references are written
// in is refused even when nothing at the declaration shadows it.
func TestRenameShadowingAtAReferenceIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::Old;\n" +
		"\tpart def Inner {\n\t\tattribute New;\n\t\tpart q : Old;\n\t}\n}\n"
	e := refusedRename(t, "shadow-ref.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "Q::Inner::New") {
		t.Fatalf("refusal does not name the shadowing feature: %s", e.Message)
	}
}

// Every reference is rewritten in one batch, so a rename with many references
// reports one applied edit per rewritten span and nothing else moves.
func TestRenameReportsEveryRewrittenSpan(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\tpart a : Old;\n\tpart b : P::Old;\n}\n"
	m := loadContent(t, "spans.sysml", src)
	requireClean(t, m)

	res := applyOne(t, m, Rename("P::Old", "Fresh"))
	if len(res.Applied) != 3 {
		t.Fatalf("applied %d edits, want 3", len(res.Applied))
	}
	for _, a := range res.Applied {
		if a.OldText != "Old" || a.NewText != "Fresh" || a.Target != "P::Old" {
			t.Fatalf("applied edit reports %+v", a)
		}
	}
	assertOnlySpanChanged(t, m, res)
}

// A constructor's argument label names a feature of the constructed type, so
// renaming that feature rewrites the label and renaming a same-named feature of
// the sender leaves it alone.
func TestRenameRewritesConstructorLabels(t *testing.T) {
	const src = "package App {\n\titem def Telemetry { attribute frames; }\n" +
		"\titem def Burst :> Telemetry;\n\tpart def Station;\n\taction def Downlink {\n" +
		"\t\tpart ground : Station;\n\t\tattribute frames;\n" +
		"\t\tsend new Telemetry(frames = 3) to ground;\n" +
		"\t\tsend new Burst(frames = frames) to ground;\n" +
		"\t\tsend new Burst(Telemetry::frames = frames) to ground;\n\t}\n}\n"

	got := renamed(t, "labels.sysml", src, "App::Telemetry::frames", "count")
	for _, want := range []string{
		"item def Telemetry { attribute count; }",
		"send new Telemetry(count = 3) to ground;",
		"send new Burst(count = frames) to ground;",
		"send new Burst(Telemetry::count = frames) to ground;",
		"\t\tattribute frames;\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}

	got = renamed(t, "labels.sysml", src, "App::Downlink::frames", "local")
	for _, want := range []string{
		"item def Telemetry { attribute frames; }",
		"\t\tattribute local;\n",
		"send new Telemetry(frames = 3) to ground;",
		"send new Burst(frames = local) to ground;",
		"send new Burst(Telemetry::frames = local) to ground;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// A transition's guard and effect see the parameter its accept declares, so
// renaming a same-named feature of the machine leaves them alone.
func TestRenameLeavesTriggerParameterReferencesAlone(t *testing.T) {
	const src = "package App {\n\titem def Request;\n\tstate def Server {\n" +
		"\t\tpart origin : Request;\n\t\tstate idle;\n\t\tstate busy;\n" +
		"\t\ttransition first idle accept origin : Request if origin != null" +
		" do send new Request() to origin then busy;\n\t}\n}\n"
	got := renamed(t, "trigger.sysml", src, "App::Server::origin", "peer")

	if !strings.Contains(got, "part peer : Request;") {
		t.Fatalf("the declaration was not renamed:\n%s", got)
	}
	if !strings.Contains(got, "accept origin : Request if origin != null do send new Request() to origin then busy;") {
		t.Fatalf("the accept's parameter or a reference to it was rewritten:\n%s", got)
	}
}

// An unnamed transition's trailing body declares its own features: renaming a
// same-named feature of the machine leaves the body's declaration and its uses
// alone, while renaming the constructed type's feature rewrites the label there.
func TestRenameSeesUnnamedTransitionBodyDeclarations(t *testing.T) {
	const src = "package App {\n\titem def Request { attribute id; }\n\tstate def Server {\n" +
		"\t\tattribute retries;\n\t\tstate idle;\n\t\tstate busy;\n" +
		"\t\ttransition first idle accept origin : Request then busy {\n" +
		"\t\t\tattribute retries;\n\t\t\tsend new Request(id = retries) to origin;\n\t\t}\n\t}\n}\n"

	got := renamed(t, "body.sysml", src, "App::Server::retries", "attempts")
	for _, want := range []string{
		"\t\tattribute attempts;\n",
		"\t\t\tattribute retries;\n\t\t\tsend new Request(id = retries) to origin;\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}

	got = renamed(t, "body.sysml", src, "App::Request::id", "key")
	for _, want := range []string{
		"item def Request { attribute key; }",
		"send new Request(key = retries) to origin;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}
