package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
)

// wantNotation asserts one warning per want, matched by code and message
// substring, and nothing else.
func wantNotation(t *testing.T, name, src, code string, wants ...string) {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if len(pd) != 0 {
		t.Fatalf("%s: extension notation must stay parsed, got errors %+v", src, pd)
	}
	got := NonstandardNotationPass{}.Run(NewContext(name, idx, pd), name, root)
	if len(got) != len(wants) {
		t.Fatalf("%s: got %d diagnostics %+v, want %d", src, len(got), got, len(wants))
	}
	for i, want := range wants {
		if got[i].Severity != SeverityWarning {
			t.Errorf("%s: severity = %v, want warning", src, got[i].Severity)
		}
		if got[i].Code != code {
			t.Errorf("%s: code = %q, want %q", src, got[i].Code, code)
		}
		if !strings.Contains(got[i].Message, want) {
			t.Errorf("%s: message = %q, want it to contain %q", src, got[i].Message, want)
		}
	}
}

// wantSilent asserts a document parses without error and carries no notation
// diagnostic: a source that failed to parse would be silent for the wrong
// reason.
func wantSilent(t *testing.T, name, src string) {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if len(pd) != 0 {
		t.Fatalf("%s: parse errors %+v", src, pd)
	}
	if got := (NonstandardNotationPass{}).Run(NewContext(name, idx, pd), name, root); len(got) != 0 {
		t.Fatalf("%s: got %+v, want no diagnostics", src, got)
	}
}

// `namespace` is KerML-only notation: both its body forms are reported in a
// SysML file, and neither is in a KerML one.
func TestNamespaceInSysMLIsKerMLNotation(t *testing.T) {
	for _, src := range []string{
		"namespace N { }",
		"namespace N;",
		"package P { namespace N; }",
	} {
		wantNotation(t, "a.sysml", src, CodeKerMLNotation, "`namespace` is KerML notation")
	}
}

// A document of no file kind — the REPL and CLI buffer — takes the SysML reading,
// so its `namespace` is reported rather than silently accepted.
func TestNamespaceInAnUnnamedDocumentIsKerMLNotation(t *testing.T) {
	wantNotation(t, "<repl>", "namespace N;", CodeKerMLNotation, "`namespace` is KerML notation")
}

func TestNamespaceInKerMLIsSilent(t *testing.T) {
	for _, src := range []string{
		"namespace N { }",
		"namespace N;",
		"package P { namespace N; }",
	} {
		wantSilent(t, "a.kerml", src)
	}
}

// `featured by` is KerML-only notation: the SysML grammar has no featuring
// clause, so a SysML file is warned and a KerML one is silent.
func TestFeaturedByInSysMLIsKerMLNotation(t *testing.T) {
	const want = "`featured by` is KerML notation"
	wantNotation(t, "a.sysml", "package P { part def A; part x featured by A; }", CodeKerMLNotation, want)
	// A list states one featuring per target, so each is reported.
	wantNotation(t, "a.sysml", "package P { part def A; part def B; part x featured by A, B; }",
		CodeKerMLNotation, want, want)
}

func TestFeaturedByInKerMLIsSilent(t *testing.T) {
	wantSilent(t, "a.kerml", "package P { class A; class B; feature x featured by A, B; }")
}

// The state notation with no production of its own is reported, one warning per
// construct.
func TestStateExtensionsAreReported(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"state def S { choice c; }", "`choice <name>;`"},
		{"state def S { junction j; }", "`junction <name>;`"},
		{"state def S { history h; }", "history"},
		{"state def S { shallow history h; }", "history"},
		{"state def S { deep history h; }", "history"},
		{"state def S { state a { defer e; } }", "`defer <event>;`"},
	} {
		wantNotation(t, "a.sysml", tc.src, CodeNonstandardNotation, tc.want)
	}
}

// A transition stating its ends with `first` and `then` is standard.
func TestStandardTransitionStaysSilent(t *testing.T) {
	wantSilent(t, "a.sysml", "state def S { state a; state b; transition first a then b; }")
}

// A `return` declares a result parameter, which is standard, and a computed
// result is the body's trailing expression, which is standard too.
func TestResultParametersAndTrailingExpressionsStaySilent(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"bare_name", "calc c { in a : Real; return a; }"},
		{"named_result_parameter", "calc c { return result : Real = 1.0; }"},
		{"trailing_literal", "calc c { in a : Real; 42 }"},
		{"trailing_arithmetic", "calc c { in basePrice : Real; basePrice * 2.0 }"},
		{"trailing_invocation", "calc c { in dx : Real; sqrt(dx*dx) }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantSilent(t, "a.sysml", tc.src)
		})
	}
}

func TestNotationWarningsPointAtTheirKeywords(t *testing.T) {
	src := `state def S {
		state a;
		choice c;
		junction j;
	}`
	root, pd, idx := analyzeInputs(t, "a.sysml", src)
	if len(pd) != 0 {
		t.Fatalf("%s: parse errors %+v", src, pd)
	}
	got := (NonstandardNotationPass{}).Run(NewContext("a.sysml", idx, pd), "a.sysml", root)
	if len(got) != 2 {
		t.Fatalf("%s: got %d diagnostics %+v, want 2", src, len(got), got)
	}
	for i, want := range []struct {
		line, length int
	}{
		{3, len("choice")},
		{4, len("junction")},
	} {
		if line := strings.Count(src[:got[i].Span.Offset], "\n") + 1; line != want.line {
			t.Errorf("diagnostic %d is on line %d, want %d", i, line, want.line)
		}
		if got[i].Span.Len != want.length {
			t.Errorf("diagnostic %d spans %d bytes, want %d", i, got[i].Span.Len, want.length)
		}
	}
}

func TestStandardConstraintAndRequirementConditionsStaySilent(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"bare_condition", "constraint validRange { in x : Real; x >= 0 }"},
		{"named_constraint_assertion", "part def P { assert constraint c1 : C; }"},
		{"satisfy_assertion", "requirement def R { assert satisfy r by q; }"},
		{"metadata_constraint_assumption", "requirement def R { assume #goal constraint payloadMassLimit; }"},
		{"named_requirement_constraint", "requirement def R { require constraint c; }"},
		{"bare_requirement_reference", "requirement def R { require c; }"},
		{"concern_bare_reference", "concern def C { require c; }"},
		{"requirement_feature_reference", "requirement r { assume x.y; }"},
		{"objective_feature_reference", "case def C { objective o { require x.y; } }"},
		{"named_assumption_with_body", "requirement def R { assume constraint c { 1 > 0 } }"},
		{"named_requirement_with_body", "requirement def R { require constraint c { 1 > 0 } }"},
		{"anonymous_assumption_body", "requirement def R { assume constraint { 1 > 0 } }"},
		{"anonymous_requirement_body", "requirement def R { require constraint { 1 > 0 } }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantSilent(t, "a.sysml", tc.src)
		})
	}
}

// Notation the pinned grammars do define stays silent: a false extension warning
// on standard notation is the worse defect.
func TestStandardNotationIsSilent(t *testing.T) {
	for _, src := range []string{
		"package P { part def Widget; }",
		"state def S { entry; then a; state a; }",
		"state def S { state a { entry action x; do action y; exit action z; } }",
		"state s parallel { state a; state b; }",
		"state def S { state a; state b; transition first a then b; }",
		"state def S { state a; state b; first a then b; }",
		"action def A { first start; then done; }",
		"action def A { fork f; join j; merge m; decide d; }",
		"action def A { action a; action b; succession a then b; }",
		"action def A { action t; action e; if true then t; else e; }",
		"action def A { while true { action a; } }",
		// The words we no longer reserve are ordinary names.
		"package P { attribute done : Boolean; attribute region : Boolean; }",
		"package P { part def final; part def history; part def defer; }",
		"package P { attribute initial = 1; attribute choice = 2; attribute junction = 3; }",
		"package P { attribute deep = 1; attribute shallow = 2; attribute decision = 3; }",
	} {
		wantSilent(t, "a.sysml", src)
	}
}

// `bind` relates two features (SysML.xtext:1020), so the standard
// feature-valued forms are silent.
func TestFeatureValuedBindingIsSilent(t *testing.T) {
	wantSilent(t, "a.sysml", "part def P { attribute a; attribute b; bind a = b; }")
	wantSilent(t, "a.sysml", "part def P { part a { attribute x; } attribute b; bind b = a.x; }")
}

// An InitialNodeMember is reachable from ActionBodyItem alone, so a
// one-ended `first` is standard in an action body and ours in a part body.
func TestOneEndedFirstOutsideAnActionBodyIsAnExtension(t *testing.T) {
	wantNotation(t, "a.sysml", "part def P { part a; first a; }",
		CodeNonstandardNotation, "one-ended `first <node>;` outside an action body")
	wantSilent(t, "a.sysml", "action def A { action a; first a; }")
	wantSilent(t, "a.sysml", "action def A { action outer { action a; first a; } }")
	wantSilent(t, "a.sysml", "part def P { action a { action b; first b; } }")
}

// F107: RequirementConstraintMember belongs to a RequirementBody, which an
// analysis case body is not.
func TestRequirementConstraintOutsideARequirementBodyIsAnExtension(t *testing.T) {
	wantNotation(t, "a.sysml", "analysis def An { attribute size; require constraint { size >= 1 } }",
		CodeNonstandardNotation, "`require` outside a requirement body")
	wantNotation(t, "a.sysml", "part def P { attribute size; assume constraint { size >= 1 } }",
		CodeNonstandardNotation, "`assume` outside a requirement body")
	wantSilent(t, "a.sysml", "requirement def R { attribute size; require constraint { size >= 1 } }")
	wantSilent(t, "a.sysml", "analysis def An { attribute size; assert constraint { size >= 1 } }")
}

// k02: the KerML grammar has no `part` declaration, so a SysML declaration
// keyword in a .kerml file is reported, and the KerML spelling is silent.
func TestSysMLDeclarationInKerMLIsReported(t *testing.T) {
	got := notationDiags(t, "a.kerml", "package P { part def Wheel; }", conformance.ModeDefault)
	if len(got) != 1 || got[0].Severity != SeverityError || got[0].Code != CodeSysMLNotation ||
		!strings.Contains(got[0].Message, "`part` is SysML notation") {
		t.Errorf("got %+v, want one sysml-notation error", got)
	}
	wantSilent(t, "a.kerml", "package P { struct Wheel; }")
	wantSilent(t, "a.sysml", "package P { part def Wheel; }")
}
