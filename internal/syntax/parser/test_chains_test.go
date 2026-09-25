package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestChainsRelationship(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{
			"chains relationship",
			`feature self: Anything[1] subsets things chains things.that;`,
		},
		{
			"chains with body",
			`feature subendshot : Occurrence [0..*] chains self.suboccurrences.endShot { }`,
		},
		{
			"feature chain chains",
			`private feature chain chains source.target;`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New("test.kerml", []byte(tc.code)))
			file := p.ParseFile()

			if len(p.Diagnostics) > 0 {
				for _, d := range p.Diagnostics {
					t.Errorf("Unexpected error: %s", d.Message)
				}
			}

			// Verify chains relationship exists
			if len(file.Members) > 0 {
				if u, ok := file.Members[0].(*ast.Usage); ok {
					found := false
					for _, rel := range u.Relationships {
						if rel.Kind == ast.RelChains {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected chains relationship in AST")
					}
				}
			}
		})
	}
}
