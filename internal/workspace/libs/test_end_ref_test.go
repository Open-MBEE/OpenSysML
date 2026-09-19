package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestEndRefPattern(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "end ref simple",
			code: `
				namespace Test {
					feature myFeature {
						end ref source;
						end ref target;
					}
				}
			`,
		},
		{
			name: "end ref in redefines body",
			code: `
				namespace Test {
					ref feature transfers {
						end ref source;
						end ref target;
					}
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.code)))
			root := p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Unexpected parse errors:")
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
				t.FailNow()
			}

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
		})
	}
}
