package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestRequirementBodiesRejectReturnParameters(t *testing.T) {
	const message = "'return' is not a member of a requirement body: only a calculation, constraint or case body declares a return parameter"
	tests := []struct {
		name string
		src  string
	}{
		{"requirement_definition", "package P { requirement def R { return q : Real; } }"},
		{"concern_definition", "package P { concern def K { return : Real; } }"},
		{"viewpoint_definition", "package P { viewpoint def W { return : Real; } }"},
		{"requirement_usage", "package P { requirement r { return : Real; } }"},
		{"satisfy_usage", "package P { requirement q; part x; satisfy requirement q by x { return : Real; } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.name+".sysml", []byte(tt.src))
			p := New(sf)
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 1 {
				t.Fatalf("got %d diagnostics, want one: %v", len(p.Diagnostics), p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if d.Message != message {
				t.Errorf("message = %q, want %q", d.Message, message)
			}
			if got := sf.Text(d.Span); got != "return" {
				t.Errorf("diagnostic sits on %q, want %q", got, "return")
			}
		})
	}
}

func TestCalculationAndCaseBodiesAcceptReturnParameters(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"calculation_definition", "package P { calc def C { return : Real; } }"},
		{"constraint_definition", "package P { constraint def K { return : Boolean; } }"},
		{"analysis_definition", "package P { analysis def A { return : Real; } }"},
		{"verification_definition", "package P { verification def V { return : Boolean; } }"},
		{"use_case_definition", "package P { use case def U { return : Real; } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
		})
	}
}
