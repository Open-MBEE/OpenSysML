package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// `initial`, `final` and `decision` no longer head an action node: the standard
// spellings are `first`, `done` and `decide`.
func TestRemovedActionNodeAliasesAreReported(t *testing.T) {
	for _, src := range []string{
		"package P { action def A { initial a; } }",
		"package P { action def A { initial a then b; action b; } }",
		"package P { action def A { final b; } }",
		"package P { action def A { decision d; } }",
	} {
		p := New(source.New("removed.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) == 0 {
			t.Errorf("%q parsed clean, want a diagnostic", src)
		}
		if dump := ast.Dump(root); strings.Contains(dump, "InitialNode") ||
			strings.Contains(dump, "FinalNode") || strings.Contains(dump, "DecisionNode") {
			t.Errorf("%q still parsed to an action node:\n%s", src, dump)
		}
	}
}

// A bare `then final;` is a succession target, so it reads as a reference to a
// member named `final` rather than as the final node it once spelled.
func TestThenFinalIsASuccessionTargetNotAFinalNode(t *testing.T) {
	root := parseClean(t, "package P { action def A { first a; action a; then final; } }")
	if dump := ast.Dump(root); strings.Contains(dump, "FinalNode") {
		t.Errorf("`then final;` still parsed to a final node:\n%s", dump)
	}
}

// The pinned grammars reserve none of the three words, so each still names a
// feature an action body declares and a succession reads.
func TestRemovedActionNodeAliasesRemainOrdinaryNames(t *testing.T) {
	for _, src := range []string{
		"package P { action def A { attribute initial : Boolean; attribute final : Boolean; attribute decision : Boolean; } }",
		"package P { action def A { action initial; action final; first initial then final; } }",
		"package P { action def A { in decision : Boolean; action a; action b; succession first a if decision then b; } }",
		// A kindless member named by an unreserved word is a feature, as it is for
		// every other such word.
		"package P { action def A { final; decision; } }",
	} {
		parseClean(t, src)
	}
}

// The standard spellings the aliases stood in for keep parsing to their nodes.
func TestStandardActionNodeSpellingsStillParse(t *testing.T) {
	root := parseClean(t, "package P { action def A { first a; action a; decide d; done; succession first a then d; succession first d then done; } }")
	dump := ast.Dump(root)
	for _, want := range []string{"InitialNode", "DecisionNode", "FinalNode"} {
		if !strings.Contains(dump, want) {
			t.Errorf("no %s in\n%s", want, dump)
		}
	}
}
