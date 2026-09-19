package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestBindingMultNameMult(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{
			"binding mult name mult",
			`binding [1] bind [0..*] base.edges = [0..*] be;`,
		},
		{
			"binding mult name mult 2",
			`binding [1] bind [0..1] tf.edges = [0..1] tfe;`,
		},
		{
			"connection connect keyword",
			`connection :MatesWith connect [1] be to [1] be;`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(tc.code)))
			_ = p.ParseFile()
			if len(p.Diagnostics) > 0 {
				for _, d := range p.Diagnostics {
					t.Errorf("Offset %d: %s", d.Span.Offset, d.Message)
				}
			}
		})
	}
}
