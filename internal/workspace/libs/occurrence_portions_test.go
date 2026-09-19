package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestPortionModifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErrs int
	}{
		{
			name: "portion feature with name and multiplicity",
			input: `package Test {
				feature Occurrence {
					portion feature all portions: Occurrence[1..*];
				}
			}`,
			wantErrs: 0,
		},
		{
			name: "portion feature with redefines",
			input: `package Test {
				feature Occurrence {
					feature matingOccurrence: Occurrence [1] {
						portion feature redefines spaceBoundary [1];
					}
				}
			}`,
			wantErrs: 0,
		},
		{
			name: "portion redefines statement",
			input: `package Test {
				feature Occurrence {
					feature portions: Occurrence;
					portion redefines portions = this.portions;
				}
			}`,
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()
			if len(p.Diagnostics) != tt.wantErrs {
				t.Errorf("got %d errors, want %d", len(p.Diagnostics), tt.wantErrs)
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}
