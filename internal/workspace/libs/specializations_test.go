package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSubsetStatement(t *testing.T) {
	input := `
		feature TimeEnclosed {
			feature laterOccurrence: Occurrence[1] subsets self;
			subset laterOccurrence.successors subsets earlierOccurrence.successors;
		}
	`

	p := parser.New(source.New("test.kerml", []byte(input)))
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}

	if root == nil {
		t.Fatal("root is nil")
	}
}

func TestDisjointStatement(t *testing.T) {
	input := `
		feature TimeDuring {
			disjoint earlierOccurrence.successors from laterOccurrence.predecessors;
		}
	`

	p := parser.New(source.New("test.kerml", []byte(input)))
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}

	if root == nil {
		t.Fatal("root is nil")
	}
}

func TestSubclassifierSpecializes(t *testing.T) {
	input := `package Test {
		subclassifier SelfLink specializes Base {
			doc
			/* SelfLink is a subtype of Base. */
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
	}
}

func TestMultipleTypingWithRedefinition(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "multiple typing in feature",
			input: `package Test {
				feature acceptedMessage : MessageTransfer, MessageAction :>> trigger;
			}`,
		},
		{
			name: "multiple typing with ref modifier",
			input: `package Test {
				ref acceptedMessage : MessageTransfer, MessageAction :>> trigger;
			}`,
		},
		{
			name: "multiple typing in body",
			input: `package Test {
				feature f {
					ref acceptedMessage : MessageTransfer, MessageAction :>> trigger;
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
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}

func TestRedefinesMultipleTargets(t *testing.T) {
	input := `package Test {
		feature f {
			private ref redefines Item::incomingTransferSort, subobjects::incomingTransferSort;
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d diagnostics:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
		}
	}
}

func TestReturnWithTwoRedefinesClauses(t *testing.T) {
	// Pattern from FeatureReferencingPerformances.kerml line 78
	input := `package Test {
		function allFeatureValues {
			in argument : Anything;
			return resultValues : Anything [*] nonunique redefines result redefines values;
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
