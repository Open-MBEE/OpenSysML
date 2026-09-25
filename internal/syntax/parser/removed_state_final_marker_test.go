package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// `final <state>;` no longer marks a state of a machine: a state body has no such
// member, so the parser diagnoses it at its own span and yields an ErrorNode
// rather than reinterpreting it as a state or a feature.
func TestRemovedStateFinalMarkerIsDiagnosed(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { state s; final s; } }",
		"package P { state S { entry; then a; state a; final a; } }",
		"package P { state def S parallel { state r { state s; final s; } } }",
		// Unterminated spelling of the same removed member.
		"package P { state def S { final s } }",
	} {
		p := New(source.New("removed.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) == 0 {
			t.Errorf("%q parsed clean, want a diagnostic", src)
			continue
		}
		for _, diag := range p.Diagnostics {
			if diag.Span.Offset == 0 && diag.Span.Len == 0 {
				t.Errorf("%q reported %q without a span", src, diag.Message)
			}
		}
		if dump := ast.Dump(root); !strings.Contains(dump, "ErrorNode") {
			t.Errorf("%q produced no ErrorNode:\n%s", src, dump)
		}
	}
}

// The pinned grammars reserve `final` nowhere, so it still names a state, a
// feature and a transition endpoint.
func TestFinalRemainsAnOrdinaryNameInAStateBody(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { attribute final : Boolean; } }",
		"package P { state def S { state final; } }",
		// A kindless member named by an unreserved word is a feature, as it is for
		// every other such word.
		"package P { state def S { final; } }",
		"package P { state S { entry; then final; state final; state other; transition first final accept go then other; } }",
	} {
		parseClean(t, src)
	}
}

// The completion the marker once spelled is a transition to `done`, which parses
// as the ordinary endpoint it is.
func TestTransitionToDoneParsesInAStateBody(t *testing.T) {
	root := parseClean(t, "package P { state S { entry; then a; state a; transition first a accept go then done; } }")
	if dump := ast.Dump(root); !strings.Contains(dump, `target="done"`) {
		t.Errorf("no transition parsed:\n%s", dump)
	}
}
