package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// A subject, actor, stakeholder, objective, state subaction or rendering written
// in a body whose grammar does not offer it is rejected with a diagnostic and an
// ErrorNode, and is not read as a generic declaration (SysML.xtext
// RequirementBodyItem, CaseBodyItem, StateBodyItem, ViewBodyItem; the pilot's
// *MembershipOwningType constraints).
func TestMemberOutsideOwningBodyIsRejected(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		keyword       string
		forbiddenNode string
	}{
		{"actor in part def", "part def P { actor a : Person; }", "actor", `kind="actor"`},
		{"actor in part usage", "part p { actor a : Person; }", "actor", `kind="actor"`},
		{"actor in action def", "action def A { actor a : Person; }", "actor", `kind="actor"`},
		{"actor in perform body", "action def A { perform action x { actor a : Person; } }", "actor", `kind="actor"`},
		{"actor in package", "package P { actor a : Person; }", "actor", `kind="actor"`},
		{"actor with visibility", "part def P { private actor a : Person; }", "actor", `kind="actor"`},
		{"actor with prefix metadata", "part def P { actor #M a : Person; }", "actor", `kind="actor"`},
		{"actor in subject body", "requirement def R { subject v : V { actor a : Person; } }", "actor", `kind="actor"`},
		{"actor in required constraint body", "requirement def R { require constraint k : K { actor a : Person; } }", "actor", `kind="actor"`},
		{"actor in rendering body", "view def V { render asTable { actor a : Person; } }", "actor", `kind="actor"`},
		{"subject in part def", "part def P { subject s : S; }", "subject", "SubjectMember"},
		{"subject in constraint def", "constraint def C { subject s : S; }", "subject", "SubjectMember"},
		{"subject in view def", "view def V { subject s : S; }", "subject", "SubjectMember"},
		{"subject in subject body", "requirement def R { subject v : V { subject w : W; } }", "subject", `name="w"`},
		{"subject in required constraint body", "requirement def R { require constraint k : K { subject w : W; } }", "subject", "SubjectMember"},
		{"subject in nested constraint body", "requirement def R { require constraint k { constraint c { subject w : W; } } }", "subject", "SubjectMember"},
		{"subject in expr body", "attribute def A { expr e { subject s : S; } }", "subject", "SubjectMember"},
		{"anonymous subject", "part def P { subject : S; }", "subject", "SubjectMember"},
		{"stakeholder in part def", "part def P { stakeholder s : Person; }", "stakeholder", `kind="stakeholder"`},
		{"stakeholder in case def", "case def C { stakeholder s : Person; }", "stakeholder", `kind="stakeholder"`},
		{"stakeholder in use case", "use case u { stakeholder s : Person; }", "stakeholder", `kind="stakeholder"`},
		{"objective in part def", "part def P { objective o { require constraint { x > 0 } } }", "objective", `kind="objective"`},
		{"objective in requirement def", "requirement def R { objective o; }", "objective", `kind="objective"`},
		{"anonymous objective in action", "action def A { objective { require constraint { x > 0 } } }", "objective", `kind="objective"`},
		{"entry action in part def", "part def P { entry action init; }", "entry", "EntryMember"},
		{"entry action at package root", "package P { entry action init; }", "entry", `name="init"`},
		{"do action at package root", "package P { do action work; }", "do", `name="work"`},
		{"exit action at package root", "package P { exit action fin; }", "exit", `name="fin"`},
		{"bare entry at file root", "entry;", "entry", "EntryMember"},
		{"entry in nested namespace", "package P { namespace N { entry init; } }", "entry", "EntryMember"},
		{"private do in library package", "library package L { private do action work; }", "do", `name="work"`},
		{"exit reference in nested package", "package P { package Q { exit fin; } }", "exit", "ExitMember"},
		{"entry reference in action def", "action def A { entry init; }", "entry", "EntryMember"},
		{"bare entry in part def", "part def P { entry; }", "entry", "EntryMember"},
		{"do action in part def", "part def P { do action run; }", "do", "DoMember"},
		{"exit action in requirement", "requirement def R { exit action stop; }", "exit", "ExitMember"},
		{"entry in case def", "case def C { entry action init; }", "entry", "EntryMember"},
		{"render in part def", "part def P { render asTable; }", "render", `kind="render"`},
		{"render rendering in part def", "part def P { render rendering r : R; }", "render", `kind="render"`},
		{"render in viewpoint", "viewpoint def V { render asTable; }", "render", `kind="render"`},
		{"render in requirement", "requirement def R { render asTable; }", "render", `kind="render"`},
		// Nested action bodies under a case or requirement are action bodies, not
		// the case or requirement body that owns these members.
		{"objective in if branch under case", "case def C { action a { if true { objective o; } } }", "objective", `kind="objective"`},
		{"actor in else branch under case", "case def C { action a { if true { x := 1; } else { actor a : Person; } } }", "actor", `kind="actor"`},
		{"subject in while body under case", "case def C { action a { while true { subject s : S; } } }", "subject", "SubjectMember"},
		{"objective in loop body under case", "case def C { action a { loop { objective o; } until true; } }", "objective", `kind="objective"`},
		{"actor in for body under case", "case def C { action a { for i in 1..3 { actor a : Person; } } }", "actor", `kind="actor"`},
		{"objective in fork body under case", "case def C { action a { fork f { objective o; } } }", "objective", `kind="objective"`},
		{"subject in decide body under case", "case def C { action a { decide d { subject s : S; } } }", "subject", "SubjectMember"},
		{"actor in merge body under case", "case def C { action a { merge m { actor a : Person; } } }", "actor", `kind="actor"`},
		{"objective in join body under case", "case def C { action a { join j { objective o; } } }", "objective", `kind="objective"`},
		{"subject in send body under case", "case def C { action a { send x to y { subject s : S; } } }", "subject", "SubjectMember"},
		{"objective in accept body under case", "case def C { action a { accept e : E { objective o; } } }", "objective", `kind="objective"`},
		{"actor in succession body under case", "case def C { action a { action b; first start then b { actor a : Person; } } }", "actor", `kind="actor"`},
		{"stakeholder in if branch under requirement", "requirement def R { action a { if true { stakeholder s : Person; } } }", "stakeholder", `kind="stakeholder"`},
		{"subject in while body under requirement", "requirement def R { action a { while true { subject s : S; } } }", "subject", "SubjectMember"},
		{"actor in fork body under requirement", "requirement def R { action a { fork f { actor a : Person; } } }", "actor", `kind="actor"`},
		{"entry in do block", "state def S { do { entry x; } }", "entry", "EntryMember"},
		{"do in exit block", "state def S { exit { do y; } }", "do", "DoMember"},
		{"objective in do block", "state def S { do { objective o; } }", "objective", `kind="objective"`},
		{"entry in transition body", "state def S { state a; state b; transition first a then b { entry q; } }", "entry", "EntryMember"},
		{"exit in transition effect block", "state def S { state a; state b; transition t first a accept e : E do { exit w; } then b; }", "exit", "ExitMember"},
		// A usage that offers no members of its own has a plain UsageBody, whatever
		// body it is written in.
		{"objective in typed usage under case", "case def C { x : T { objective o; } }", "objective", `kind="objective"`},
		{"objective in bare usage under case", "case def C { x { objective o; } }", "objective", `kind="objective"`},
		{"objective in ref usage under case", "case def C { ref x : T { objective o; } }", "objective", `kind="objective"`},
		{"objective in in parameter under case", "case def C { in x : T { objective o; } }", "objective", `kind="objective"`},
		{"objective in return parameter under case", "case def C { return x : T { objective o; } }", "objective", `kind="objective"`},
		{"objective in redefinition under case", "case def C { :>> x { objective o; } }", "objective", `kind="objective"`},
		{"objective in anonymous end under case", "case def C { end ref { objective o; } }", "objective", `kind="objective"`},
		{"objective in metadata usage under case", "case def C { @M { objective o; } }", "objective", `kind="objective"`},
		{"objective in body expression under case", "case def C { attribute x = { in y; objective o; y }; }", "objective", `kind="objective"`},
		{"objective in succession body under case", "case def C { first a then b { objective o; } }", "objective", `kind="objective"`},
		{"objective in subject body under case", "case def C { subject s { objective o; } }", "objective", `kind="objective"`},
		{"objective in part usage under case", "case def C { part p : T { objective o; } }", "objective", `kind="objective"`},
		{"objective in typed usage under analysis case", "analysis def A { x : T { objective o; } }", "objective", `kind="objective"`},
		{"stakeholder in typed usage under requirement", "requirement def R { x : T { stakeholder s; } }", "stakeholder", `kind="stakeholder"`},
		{"stakeholder in bare usage under requirement", "requirement def R { x { stakeholder s; } }", "stakeholder", `kind="stakeholder"`},
		{"actor in typed usage under requirement", "requirement def R { x : T { actor a; } }", "actor", `kind="actor"`},
		{"subject in typed usage under requirement", "requirement def R { x : T { subject s; } }", "subject", "SubjectMember"},
		{"stakeholder in stakeholder body", "requirement def R { stakeholder s { stakeholder t; } }", "stakeholder", `name="t"`},
		{"entry in typed usage under state", "state def S { x : T { entry; } }", "entry", "EntryMember"},
		{"entry action in bare usage under state", "state def S { x { entry action a; } }", "entry", "EntryMember"},
		{"do action in typed usage under state", "state def S { x : T { do action a; } }", "do", "DoMember"},
		{"exit in entry action body", "state def S { entry action a { exit; } }", "exit", "ExitMember"},
		{"render in typed usage under view", "view def V { x : T { render r; } }", "render", `kind="render"`},
		{"render in bare usage under view", "view def V { x { render r; } }", "render", `kind="render"`},
		{"render in in parameter under view", "view def V { in x : T { render r; } }", "render", `kind="render"`},
		{"render in rendering body", "view def V { render r { render q; } }", "render", `name="q"`},
		{"render in part usage under view", "view def V { part p { render r; } }", "render", `kind="render"`},
		{"render in typed usage under nested view", "view def V { view v { x { render r; } } }", "render", `kind="render"`},
		{"render in typed usage under view usage", "view v { x : T { render r; } }", "render", `kind="render"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New("t.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			want := "'" + tt.keyword + "' declares "
			var found bool
			for _, d := range p.Diagnostics {
				if strings.Contains(d.Message, want) && strings.Contains(d.Message, "only allowed in a") {
					found = true
				}
			}
			if !found {
				t.Fatalf("no owning-body diagnostic for %q: %v", tt.keyword, p.Diagnostics)
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", dump)
			}
			if strings.Contains(dump, tt.forbiddenNode) {
				t.Fatalf("misplaced member kept as %s:\n%s", tt.forbiddenNode, dump)
			}
			if len(p.Diagnostics) != 1 {
				t.Fatalf("want the owning-body diagnostic alone, got %v", p.Diagnostics)
			}
		})
	}
}

// KerML reserves none of the member keywords, so each names an ordinary feature
// in a `.kerml` file, in every shape the KerML Feature production offers.
func TestOwnedMemberKeywordsAreKerMLNames(t *testing.T) {
	for _, word := range []string{"actor", "subject", "stakeholder", "objective", "entry", "do", "exit", "render"} {
		for _, shape := range []string{
			"feature %s : A;",
			"feature %s;",
			"%s : A;",
			"%s;",
			"%s [1] : A;",
			"%s :> other;",
			"in %s : A;",
			"%s = 1;",
			"feature %s { feature nested; }",
			"%s : A { feature nested; }",
			"class C { %s : A; feature %s; }",
		} {
			decl := strings.ReplaceAll(shape, "%s", word)
			src := "package P { class A; feature other; " + decl + " }"
			t.Run(decl, func(t *testing.T) {
				p := New(source.New("t.kerml", []byte(src)))
				root := p.ParseFile()
				if len(p.Diagnostics) != 0 || len(p.Warnings) != 0 {
					t.Fatalf("errors = %v, warnings = %v, want none", p.Diagnostics, p.Warnings)
				}
				dump := ast.Dump(root)
				if !strings.Contains(dump, `name="`+word+`"`) {
					t.Fatalf("no feature named %q:\n%s", word, dump)
				}
				if strings.Contains(dump, "ErrorNode") || strings.Contains(dump, `kind="`+word+`"`) {
					t.Fatalf("%q read as a member keyword:\n%s", word, dump)
				}
			})
		}
	}
}

// The same members parse without a diagnostic in every body kind that offers
// them, in their definition and usage forms, and the state shorthands and
// transitions inside a state body are untouched.
func TestMemberInsideOwningBodyStaysClean(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"requirement def", "requirement def R { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"requirement usage", "requirement r { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"nested requirement", "requirement def R { requirement sub { actor a : Person; stakeholder s : Person; } }"},
		{"concern def", "concern def C { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"viewpoint usage", "viewpoint vp { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"satisfy body", "part p { satisfy r by p { subject v : V; actor a : Person; } }"},
		{"frame body", "requirement def R { frame c { stakeholder s : Person; actor a : Person; } }"},
		{"objective body", "case def C { objective o { subject v : V; actor a : Person; stakeholder s : Person; } }"},
		{"case def", "case def C { subject v : V; actor a : Person; objective o; }"},
		{"case usage", "case c { subject v : V; actor a : Person; objective { require constraint { x > 0 } } }"},
		{"analysis case def", "analysis def A { subject v : V; actor a : Person; objective o; }"},
		{"verification case usage", "verification v { subject v : V; actor a : Person; objective o; }"},
		{"use case def", "use case def U { subject v : V; actor a : Person; objective o; }"},
		{"include use case", "use case def U { include use case u { subject v : V; actor a : Person; } }"},
		{"include reference body", "use case def U { include u { subject v : V; actor a : Person; objective o; } }"},
		{"perform reference body", "part def P { perform a { first x; action x; } }"},
		{"view def", "view def V { render asTable; }"},
		{"view usage", "view v { render asTable; }"},
		{"view rendering declaration", "view v { render rendering r : R; }"},
		{"state def subactions", "state def S { entry action init; do action run; exit action stop; }"},
		{"state usage subactions", "state s { entry init; do run; exit stop; }"},
		{"state shorthands", "state def S { entry; do; exit; state a; state b; }"},
		{"entry then", "state def S { entry; then a; state a; }"},
		{"entry transition", "state def S { entry; then a; state a; transition a_to_b first a then b; state b; }"},
		{"inline state actions", "state def S { entry send x to y; do assign a := 1; exit send z to y; }"},
		{"braced state actions", "state def S { entry action { x := 1; } do { y := 2; } exit action { z := 3; } }"},
		{"nested state", "state def S { state inner { entry action init; state deeper { entry; exit action stop; } } }"},
		{"exhibit state body", "part def P { exhibit state s { entry action init; do action run; } }"},
		{"exhibit reference body", "part def P { exhibit s { entry; do action run; } }"},
		{"parallel state", "state def S parallel { state a { entry action init; } state b { exit; } }"},
		{"keywords as names", "package P { attribute actor; attribute subject; part render; }"},
		{"nested view keeps its body", "view def V { view v { render r; } }"},
		{"nested action bodies under case", "case def C { subject s : S; action a { if true { x := 1; } else { y := 2; } while true { z := 3; } for i in 1..3 { action n; } fork f { action g; } send x to y { in item q; } } }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			if p.ParseFile() == nil {
				t.Fatal("nil root")
			}
			for _, d := range p.Diagnostics {
				t.Errorf("unexpected diagnostic: %s", d.Message)
			}
		})
	}
}
