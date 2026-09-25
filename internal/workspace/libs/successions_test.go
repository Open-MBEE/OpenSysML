package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSuccessionIdentifierMultiplicity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErrs int
	}{
		{
			name: "succession with identifier multiplicity",
			input: `package Test {
				feature accNum: Natural;
				connector TransitionLink {
					private succession [accNum] accept then [1] target;
				}
			}`,
			wantErrs: 0,
		},
		{
			name: "succession with feature reference multiplicity",
			input: `package Test {
				feature seBeforeNum: Natural;
				connector Flow {
					succession [seBeforeNum] first [0..1] source then [0..1] target;
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

func TestSuccessionNamedIdentifierMultiplicity(t *testing.T) {
	input := `package Test {
		feature taNum: Natural [1] = 5;
		
		succession triggerAfter [taNum] first [0..1] a then [*] b;
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
