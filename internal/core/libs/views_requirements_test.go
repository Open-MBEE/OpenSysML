package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestRequireInRequirementAndSatisfyBody(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "require in requirement body",
			input: `package Test {
				requirement r {
					require viewpointSatisfactions {
						doc /* constraint */
					}
				}
			}`,
		},
		{
			name: "require in satisfy body",
			input: `package Test {
				satisfy requirement viewpointConformance by that {
					require viewpointSatisfactions {
						doc /* constraint */
					}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d diagnostics:", len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
				}
			}
		})
	}
}

func TestViewSatisfyRequireStructure(t *testing.T) {
	// Exact structure from Views.sysml lines 40-50
	input := `package Views {
		view def View {
			satisfy requirement viewpointConformance by that {
				require viewpointSatisfactions {
					doc
					/*
					 * The required ViewpointChecks.
					 */
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(input) {
			char = string(input[byteOffset])
		}
		t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}

func TestRefRedefinesInFeatureBody(t *testing.T) {
	input := `package Test {
		feature viewpointSatisfactions {
			ref :>> ownedPerformances::this, subperformances::this default that.that;
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(input) {
			char = string(input[byteOffset])
		}
		t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}

func TestRefRedefinesInBodyVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "ref in feature body",
			input: `package Test {
				feature x {
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}`,
		},
		{
			name: "ref in constraint body",
			input: `package Test {
				constraint x {
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}`,
		},
		{
			name: "ref in require body",
			input: `package Test {
				require x {
					ref :>> ownedPerformances::this, subperformances::this default that.that;
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.input)))
			_ = p.ParseFile()

			t.Logf("Diagnostics: %d", len(p.Diagnostics))
			for _, d := range p.Diagnostics {
				t.Logf("  - %s", d.Message)
			}

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
			}
		})
	}
}
