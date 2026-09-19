package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A qualified name separates segments with `::` and a feature chain with `.`, so
// `A..B` chains over a segment that is not written (KerML.xtext:399
// QualifiedName, :424 OwnedFeatureChaining). The diagnostic belongs on the `..`
// that is written, and recovery stays inside the reference: the declaration it
// appears in still ends at its `;`, so the rest of the body parses.
func TestChainDoubleDotReportedAtTheDots(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"two_dots", "package test {\n\tclassifier A { feature a; }\n\tclassifier B {\n\t\tfeature b : test..A::a;\n\t\tfeature c : A;\n\t}\n}\n"},
		{"two_dots_at_the_end", "package test {\n\tclassifier A { feature a; }\n\tclassifier B {\n\t\tfeature b : test::A..a;\n\t\tfeature c : A;\n\t}\n}\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".kerml", []byte(tt.src)))
			root := p.ParseFile()

			if len(p.Diagnostics) != 1 {
				for _, d := range p.Diagnostics {
					t.Logf("offset %d: %s", d.Span.Offset, d.Message)
				}
				t.Fatalf("got %d diagnostics, want 1", len(p.Diagnostics))
			}
			d := p.Diagnostics[0]
			if want := strings.Index(tt.src, ".."); d.Span.Offset != want {
				t.Errorf("diagnostic at offset %d (%q), want %d (the '..')",
					d.Span.Offset, tt.src[d.Span.Offset:d.Span.Offset+d.Span.Len], want)
			}

			// Recovery kept the enclosing body: the declaration after the
			// malformed one is still a member of classifier B.
			dump := ast.Dump(root)
			if !strings.Contains(dump, `name="c"`) {
				t.Errorf("declaration after the malformed reference was dropped:\n%s", dump)
			}
		})
	}
}
