package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestFunctionParameterMultiplicity(t *testing.T) {
	// Pattern from Occurrences.kerml
	input := `package Test {
		function before {
			in t1: Transfer [1];
			in t2: Transfer [1];
			return t1First: Boolean [1];
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

func TestReturnAssignmentInFunctionBody(t *testing.T) {
	// Simplest test - return statement inside feature body
	input := `package Test {
		function test {
			return result = 42;
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Diagnostics: %d", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			byteOffset := d.Span.Offset
			char := ""
			if byteOffset < len(input) {
				char = string(input[byteOffset])
			}
			t.Logf("  offset=%d (char=%q): %s", byteOffset, char, d.Message)
		}
		t.Errorf("Expected clean parse")
	}
}

func TestReturnAssignmentInBoolBody(t *testing.T) {
	// Bool usage with return statement inside body
	input := `package Test {
		bool test {
			return result = true;
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Diagnostics: %d", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			byteOffset := d.Span.Offset
			char := ""
			if byteOffset < len(input) {
				char = string(input[byteOffset])
			}
			t.Logf("  offset=%d (char=%q): %s", byteOffset, char, d.Message)
		}
		t.Errorf("Expected clean parse")
	}
}

func TestReturnDeclarationWithAssignment(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "return with assignment",
			input: `package Test {
				function includes {
					in seq: Integer[*];
					in elem: Integer;
					return result: Boolean = true;
				}
			}`,
		},
		{
			name: "return with expression assignment",
			input: `package Test {
				function earlierFirstIncomingTransferSort {
					in t1: Transfer [1];
					in t2: Transfer [1];
					return t1First: Boolean = includes(t1.endShot.successors, t2.endShot);
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}

func TestBoolPredicateReturnBinding(t *testing.T) {
	// Exact pattern from Occurrences.kerml line 702-706
	input := `package Test {
		function includes {
			in seq: Integer[*];
			in elem: Integer;
			return result: Boolean;
		}
		
		struct Transfer {
			feature endShot: Integer;
		}
		
		struct IncomingTransferSort {
			in t1: Transfer [1];
			in t2: Transfer [1];
			return t1First: Boolean [1];
		}
		
		bool earlierFirstIncomingTransferSort : IncomingTransferSort {
			return t1First = includes(t1.endShot.successors, t2.endShot);
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Diagnostics: %d", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			byteOffset := d.Span.Offset
			char := ""
			if byteOffset < len(input) {
				char = string(input[byteOffset])
			}
			t.Logf("  offset=%d (char=%q): %s", byteOffset, char, d.Message)
		}
		t.Errorf("Expected clean parse")
	}
}

func TestReturnDeclarationWithTyping(t *testing.T) {
	// Test 1: return with assignment (already works)
	input1 := `package Test {
		bool test1 {
			return result = 5;
		}
	}`

	p1 := parser.New(source.New("test1.kerml", []byte(input1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (return with assignment) - Diagnostics: %d", len(p1.Diagnostics))

	// Test 2: return with typing only (fails)
	input2 := `package Test {
		predicate IncomingTransferSort {
			in t1: Transfer [1];
			return t1First: Boolean [1];
		}
	}`

	p2 := parser.New(source.New("test2.kerml", []byte(input2)))
	_ = p2.ParseFile()
	t.Logf("Test 2 (return with typing only) - Diagnostics: %d", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}

	if len(p2.Diagnostics) > 0 {
		t.Errorf("Expected clean parse for return declaration, got %d errors", len(p2.Diagnostics))
	}
}

func TestAnonymousReturnAfterDoc(t *testing.T) {
	// Exact pattern from Performances.kerml with doc comment
	input := `package Test {
		abstract predicate BooleanEvaluation {
			doc
			/*
			 * Test doc comment.
			 */
		 
			return : Boolean[1];
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(input) {
			char = string(input[d.Span.Offset])
		}
		t.Logf("  - offset=%d (char=%q): %s", d.Span.Offset, char, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
