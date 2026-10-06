package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const multiplicityBoundFormMessage = "a multiplicity bound must be a literal or a feature name (KerML.xtext MultiplicityExpressionMember)"
const multiplicityBoundPrefixMessage = "a multiplicity bound cannot start with '-': a bound is a literal or a feature name (KerML.xtext MultiplicityExpressionMember)"

func TestMultiplicityBoundGrammar(t *testing.T) {
	tests := []struct {
		name            string
		bound           string
		wantMessage     string
		wantPrefix      string
		wantDiagnostics int
	}{
		{"multiplicity_arithmetic_bound", "n+1", multiplicityBoundFormMessage, "n+1", 1},
		{"parenthesised", "(n)", multiplicityBoundFormMessage, "(n)", 1},
		{"feature_chain", "a.b", multiplicityBoundFormMessage, "a.b", 1},
		{"cast", "0..(2 as ScalarValues::Integer)", multiplicityBoundFormMessage, "(2 as", 1},
		{"multiplicative_upper", "1..2*3", multiplicityBoundFormMessage, "2*3", 1},
		{"invocation", "F()", multiplicityBoundFormMessage, "F()", 1},
		{"null", "null", multiplicityBoundFormMessage, "null", 1},
		{"negative", "-1", multiplicityBoundPrefixMessage, "-1", 1},
		{"qualified_name", "P::n", "", "", 0},
		{"name", "n", "", "", 0},
		{"range_with_name", "0..n", "", "", 0},
		{"infinity", "*", "", "", 0},
		{"range_infinity", "0..*", "", "", 0},
		{"integer", "1", "", "", 0},
		{"real", "1.5", "", "", 0},
		{"string", `"x"`, "", "", 0},
		{"boolean", "true", "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package P { attribute n : ScalarValues::Natural = 2; part def A { attribute b : ScalarValues::Natural; } part a : A; part def D; part d [" + tt.bound + "] : D; }"
			p := New(source.New("a.sysml", []byte(src)))
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != tt.wantDiagnostics {
				t.Fatalf("got %d parser diagnostics, want %d: %+v", len(p.Diagnostics), tt.wantDiagnostics, p.Diagnostics)
			}
			if tt.wantMessage == "" {
				return
			}
			d := p.Diagnostics[0]
			if d.Message != tt.wantMessage {
				t.Fatalf("diagnostic message = %q, want %q", d.Message, tt.wantMessage)
			}
			if got := src[d.Span.Offset:]; !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("diagnostic starts at %q, want prefix %q", got, tt.wantPrefix)
			}
		})
	}

	t.Run("KerML", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			bound string
			want  int
		}{
			{"arithmetic", "n+1", 2},
			{"qualified_name", "P::n", 0},
		} {
			t.Run(tt.name, func(t *testing.T) {
				src := "package P { feature n : ScalarValues::Natural = 2; feature d [" + tt.bound + "]; multiplicity m [" + tt.bound + "]; }"
				p := New(source.New("a.kerml", []byte(src)))
				if p.ParseFile() == nil {
					t.Fatal("ParseFile returned nil")
				}
				got := 0
				for _, d := range p.Diagnostics {
					if d.Message == multiplicityBoundFormMessage {
						got++
					}
				}
				if got != tt.want {
					t.Errorf("got %d multiplicity-bound grammar diagnostics, want %d: %+v", got, tt.want, p.Diagnostics)
				}
			})
		}
	})
}
