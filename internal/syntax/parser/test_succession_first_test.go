package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSuccessionFirstThen(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{
			"succession with first/then",
			`private succession causalOrdering first [1] causes.startShot then [1] effects;`,
		},
		{
			"succession named without first",
			`succession s a then b;`,
		},
		{
			"succession anonymous",
			`succession [1] ifTest then [0..1] thenClause;`,
		},
		{
			"succession with first and multiplicity",
			`succession s first [nCauses] causes.startShot then [nEffects] effects { }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New("test.kerml", []byte(tc.code)))
			_ = p.ParseFile()
			if len(p.Diagnostics) > 0 {
				for _, d := range p.Diagnostics {
					t.Errorf("Offset %d: %s", d.Span.Offset, d.Message)
				}
			}
		})
	}
}
