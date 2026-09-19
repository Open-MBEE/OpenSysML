package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// w6cDiags returns the codes and messages src produces, for reproducers whose
// matched pinned-validator verdict is quoted in each test's comment.
func w6cDiags(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	return Analyze(name, root, pd, idx)
}

func w6cCodes(diags []diag.Diagnostic) string {
	var out []string
	for _, d := range diags {
		out = append(out, fmt.Sprintf("%v/%s", d.Severity, d.Code))
	}
	return strings.Join(out, " ")
}

// Row ~913. `var a : Amount;` with the kind keyword left out parses: the pinned
// KerML validator accepts it too, reporting only `Must be owned by an
// occurrence type`, a typing rule we do not implement.
func TestW6CVarWithoutAKindKeywordParses(t *testing.T) {
	got := w6cDiags(t, "w6c_var.kerml", `package P {
	datatype Amount;
	class C {
		var a : Amount;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// Row ~913. `var` and `on` are positional in the pinned grammars, so both spell
// a name; the pinned SysML validator accepts this fixture and so do we.
func TestW6CContextualKeywordsInNamePosition(t *testing.T) {
	got := w6cDiags(t, "w6c_contextual.sysml", `package P {
	attribute def A;
	part def Q {
		attribute on : A;
		attribute var : A;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %s, want no diagnostics", w6cCodes(got))
	}
}

// Row ~949. A misspelled transition endpoint is unresolved, as the pinned
// validator has it (`Couldn't resolve reference to Feature 'bb'`; its second
// finding, `A transition must own a succession to its target`, is a structural
// rule of another tier).
func TestW6CTransitionEndpointNameIsResolved(t *testing.T) {
	got := w6cDiags(t, "w6c_transition.sysml", `package P {
	state def S {
		state a;
		state b;
		transition first a then bb;
	}
}`)
	if len(got) != 1 || got[0].Code != "unresolved" {
		t.Fatalf("got %+v, want one unresolved-reference diagnostic", got)
	}
}

// Row ~950, a stated gap. The pinned validator resolves a trigger name as a
// type reference and reports `Couldn't resolve reference to Type 'sigX'`; we
// treat a bare trigger as an injected event and stay silent.
func TestW6CSignalTriggerNameIsNotResolved(t *testing.T) {
	got := w6cDiags(t, "w6c_trigger.sysml", `package P {
	state def S {
		state a;
		state b;
		transition first a accept sigX then b;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want the documented silence", got)
	}
}

// Row ~950, the half that is refereeable. A named payload declares a parameter,
// so its typing is a reference: the pinned validator reports `Couldn't resolve
// reference to Type 'Undeclared'` on this fixture and so do we.
func TestW6CAcceptPayloadTypingIsResolved(t *testing.T) {
	got := w6cDiags(t, "w6c_payload.sysml", `package P {
	state def S {
		state a;
		state b;
		transition first a accept m : Undeclared then b;
	}
}`)
	if len(got) != 1 || got[0].Code != "unresolved" {
		t.Fatalf("got %s, want one unresolved-reference diagnostic", w6cCodes(got))
	}
}

// Rows ~943 and ~947. A flow stating feature-chain ends, and the `individual`
// and `snapshot` modifiers, resolve clean here and in the pinned validator.
func TestW6CFlowChainEndsAndOccurrenceModifiers(t *testing.T) {
	got := w6cDiags(t, "w6c_flow.sysml", `package P {
	attribute def Amount;
	port def Pt { out attribute o : Amount; in attribute i : Amount; }
	part def Q2 { port p : Pt; }
	part def Q {
		part a : Q2;
		part b : Q2;
		flow f from a.p.o to b.p.i;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %s, want no diagnostics", w6cCodes(got))
	}
	got = w6cDiags(t, "w6c_individual.sysml", `package P {
	occurrence def Flight;
	individual occurrence def F1 :> Flight;
	part def Q {
		snapshot occurrence takeoff : F1;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %s, want no diagnostics", w6cCodes(got))
	}
}

// Row ~952, a stated gap. The pinned validator rejects an accept parameter read
// from a sibling action node (`Couldn't resolve reference to Element 'msg'`);
// our scoping resolves it, matching the executor's shared feature space.
func TestW6CAcceptParameterIsVisibleToSiblingNodes(t *testing.T) {
	got := w6cDiags(t, "w6c_accept.sysml", `package P {
	attribute def Text;
	attribute def Msg { attribute payload : Text; }
	action def A {
		accept msg : Msg;
		action consume { attribute z = msg.payload; }
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want the documented silence", got)
	}
}

// Row ~964. An expose imports all elements regardless of visibility: exposing a
// private member is clean in the pinned validator and in ours.
func TestW6CExposeOfAPrivateMemberIsClean(t *testing.T) {
	got := w6cDiags(t, "w6c_expose.sysml", `package P {
	package Inner {
		private part def Hidden;
	}
	view def V;
	view v : V {
		expose Inner::Hidden;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// Row ~982. A succession usage is a redefinition target: the pinned validator
// and we are both clean on the corrected fixture, so the F52 diagnostic no
// longer reproduces; the `SymbolUnknown` root cause is in `internal/core/symbols`.
func TestW6CSuccessionRedefinitionTargetIsClean(t *testing.T) {
	got := w6cDiags(t, "w6c_succession.sysml", `package P {
	action def A {
		action a;
		action b;
		succession named : A first a then b;
	}
	action def B :> A {
		succession redefines named first a then b;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// Row ~960, the import case. D2 raised it from a warning to an error in the
// default mode: ImportPrefix makes the indicator mandatory, so the reference
// rejects it too (`mismatched input 'import' expecting '}'`).
func TestW6CImportWithoutVisibilityIsAnError(t *testing.T) {
	got := w6cDiags(t, "w6c_import.sysml", "package P { import Q::*; package Q { attribute def A; } }")
	for _, d := range got {
		if d.Code == "import-visibility" && d.Severity == diag.SeverityError {
			return
		}
	}
	t.Fatalf("got %s, want an import-visibility error", w6cCodes(got))
}

// Row ~960, the negative row. Each notation below is an OpenSysML extension no
// pinned production admits: the reference reports a syntax error
// (`mismatched input 'of' expecting 'bind'`, `no viable alternative at input
// 'N'`, `no viable alternative at input 'featured'`), while we accept it with a
// warning naming the language.
func TestW6CNotationNoPinnedProductionAdmitsIsWarned(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
		code string
	}{
		{
			name: "binding of clause",
			file: "w6c_binding_of.sysml",
			src: `package P {
	attribute def Flag;
	attribute full : Flag;
	attribute level : Flag;
	binding bb of full = level;
}`,
			code: "",
		},
		{
			name: "namespace in sysml",
			file: "w6c_namespace.sysml",
			src:  "namespace N { part def Q; }",
			code: "kerml-notation",
		},
		{
			name: "featured by in sysml",
			file: "w6c_featured.sysml",
			src: `package P {
	attribute def Flag;
	part def Q {
		attribute a : Flag;
		part x : Q featured by a;
	}
}`,
			code: "kerml-notation",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := w6cDiags(t, tc.file, tc.src)
			for _, d := range got {
				if d.Severity == diag.SeverityError {
					t.Fatalf("got %+v, want no error for accepted notation", got)
				}
			}
			if tc.code == "" {
				return
			}
			found := false
			for _, d := range got {
				if d.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("got %s, want a %s warning", w6cCodes(got), tc.code)
			}
		})
	}
}
