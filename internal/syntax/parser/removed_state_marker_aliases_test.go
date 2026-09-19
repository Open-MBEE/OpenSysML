package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// `initial <state>;` no longer marks the state a machine starts in, and a
// transition no longer states its ends with `to`.
func TestRemovedStateMarkerAliasesAreReported(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { initial a; } }",
		"package P { state def S { state a; state b; transition a to b; } }",
		"package P { state def S { state a; state b; transition t a to b; } }",
		"package P { state def S { region r { initial a; } } }",
	} {
		p := New(source.New("removed.sysml", []byte(src)))
		p.ParseFile()
		if len(p.Diagnostics) == 0 {
			t.Errorf("%q parsed clean, want a diagnostic", src)
		}
	}
}

// The removed transition spelling is not read as a transition between the two
// states it named.
func TestTransitionToIsNotATransitionBetweenItsEnds(t *testing.T) {
	src := "package P { state def S { state a; state b; transition a to b; } }"
	p := New(source.New("removed.sysml", []byte(src)))
	if dump := ast.Dump(p.ParseFile()); strings.Contains(dump, "TransitionMember") {
		t.Errorf("`transition a to b;` still parsed to a transition:\n%s", dump)
	}
}

// No pinned grammar reserves `initial`, so it still names a feature a state
// body declares. `to` stays reserved: it is a literal of the connector,
// interface, message and flow ends.
func TestRemovedStateMarkerWordsRemainOrdinaryNames(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { attribute initial : Boolean; } }",
		"package P { state def S { state initial; state b; transition first initial then b; } }",
		"package P { state def S { in initial : Boolean; state a; state b; transition first a if initial then b; } }",
	} {
		parseClean(t, src)
	}
}

// The standard spellings the aliases stood in for keep parsing.
func TestStandardStateMarkerSpellingsStillParse(t *testing.T) {
	root := parseClean(t, "package P { state def S { entry; then a; state a; state b; transition first a then b; } }")
	dump := ast.Dump(root)
	for _, want := range []string{"EntryMember", "TransitionMember"} {
		if !strings.Contains(dump, want) {
			t.Errorf("standard spelling lost %s:\n%s", want, dump)
		}
	}
}
