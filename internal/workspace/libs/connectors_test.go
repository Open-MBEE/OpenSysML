package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestFeatureChainConnectorEnd(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "succession with feature chain ends",
			code: `
				state TestState {
					succession [1] do.startShot then [*] nonDoMiddle.startShot;
				}
			`,
		},
		{
			name: "connection with feature chain ends",
			code: `
				part TestPart {
					connection connect [1] source.port to [1] target.port;
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := source.New("test.kerml", []byte(tt.code))
			p := parser.New(file)
			root := p.ParseFile()

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}

			for _, d := range p.Diagnostics {
				t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
			}
		})
	}
}

func TestConnectorTypingAndEnds(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "connector with 'to' typing",
			input: `package Test {
				private connector [0..1] transitionLink to [1..*] trigger;
			}`,
		},
		{
			name: "connector with standard typing",
			input: `package Test {
				private connector [0..1] transitionLink : TriggerType;
			}`,
		},
		{
			name: "connector with from/to ends",
			input: `package Test {
				private connector transitionLink from [0..1] source to [1..*] target;
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
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

func TestAssociationEndWithSubsets(t *testing.T) {
	input := `package Test {
		assoc HappensWhile {
			end happensWhile [1..*] subsets timeCoincidentOccurrences feature thatOccurrence: Occurrence;
		}
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

func TestSuccessionAndConnectorLeadingMultiplicity(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantClean bool
	}{
		{
			name:      "succession_mult_name_first",
			input:     `package Test { succession [1] mySuccession first [1] a then [1] b; }`,
			wantClean: true,
		},
		{
			name:      "succession_mult_first_anonymous",
			input:     `package Test { succession [1] first [1] a then [1] b; }`,
			wantClean: true,
		},
		{
			name:      "connector_mult_name",
			input:     `package Test { connector [1] myConn from [1] a to [1] b; }`,
			wantClean: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.input)))
			_ = p.ParseFile()

			clean := len(p.Diagnostics) == 0
			if clean != tt.wantClean {
				t.Errorf("wantClean=%v, got=%v", tt.wantClean, clean)
				for _, d := range p.Diagnostics {
					t.Logf("  offset %d: %s", d.Span.Offset, d.Message)
				}
			}
		})
	}
}
