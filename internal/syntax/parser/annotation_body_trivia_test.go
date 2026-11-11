package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// leadingTriviaByName maps each named member of a parse to the text of its
// leading trivia, "" for a member with none.
func leadingTriviaByName(t *testing.T, input string) map[string][]string {
	t.Helper()
	sf := source.New("a.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	got := map[string][]string{}
	ast.Inspect(root, func(n ast.Node) bool {
		m, ok := n.(*ast.Membership)
		if !ok {
			return true
		}
		name := ""
		switch d := m.Member.(type) {
		case *ast.Definition:
			name = d.Ident.Name
		case *ast.Usage:
			name = d.Ident.Name
		case *ast.Package:
			name = d.Ident.Name
		}
		if name == "" {
			return true
		}
		texts := []string{}
		for _, tr := range m.LeadingTrivia() {
			texts = append(texts, strings.TrimSpace(sf.Text(tr.Span)))
		}
		got[name] = texts
		return true
	})
	return got
}

// The body of a `doc`, `comment` or `rep` belongs to that annotation; the
// declaration parsed after it, at any depth, is not handed it as leading trivia.
func TestAnnotationBodyIsNotTheNextDeclarationsTrivia(t *testing.T) {
	src := `package HoverRepro {
    attribute def MatingInterface {
        doc /* DOC_FOR_MATING_INTERFACE */
        attribute interfaceName {
            doc /* DOC_FOR_INTERFACE_NAME */
        }
    }
    attribute def ConnectorFamily {
        doc /* DOC_FOR_CONNECTOR_FAMILY */
        doc second /* SECOND_DOC */
        comment /* OWNED_COMMENT */
        comment c about familyName /* NAMED_COMMENT */
        rep inOcl language "ocl" /* REP_BODY */
        attribute familyName {
            doc /* DOC_FOR_FAMILY_NAME */
        }
    }
    attribute sample : ConnectorFamily;
    // a note
    //* a block note */
    attribute noted : ConnectorFamily;
}
`
	got := leadingTriviaByName(t, src)
	want := map[string][]string{
		"HoverRepro":      {},
		"MatingInterface": {},
		"interfaceName":   {},
		"ConnectorFamily": {},
		"familyName":      {},
		"sample":          {},
		"noted":           {"// a note", "//* a block note */"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("leading trivia = %v, want %v", got, want)
	}
}

// A bare comment stays trivia (it is a Comment of the enclosing namespace), even
// one written right after a `doc` body: only the consumed body leaves the trivia.
func TestOnlyTheConsumedBodyLeavesThePendingTrivia(t *testing.T) {
	src := "package P {\n    doc /* d */ /* c */\n    part def Wheel;\n    /* above */\n    part def Axle;\n}\n"
	got := leadingTriviaByName(t, src)
	want := map[string][]string{
		"P":     {},
		"Wheel": {"/* c */"},
		"Axle":  {"/* above */"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("leading trivia = %v, want %v", got, want)
	}
}

// A body consumed inside an abandoned parse attempt is replayed on restore and
// consumed again by the attempt that succeeds, so it never surfaces as trivia.
func TestAnnotationBodyStaysOutOfTriviaAcrossBacktracking(t *testing.T) {
	src := "package P {\n    part def Car {\n        doc /* a car */\n        attribute mass : Real = 1.0;\n    }\n    part def Bus { doc /* a bus */ }\n    part def Van;\n}\n"
	got := leadingTriviaByName(t, src)
	for name, texts := range got {
		if len(texts) != 0 {
			t.Errorf("%s leading trivia = %q, want none", name, texts)
		}
	}
}
