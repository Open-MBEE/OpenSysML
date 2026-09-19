package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestInvariantBodyExpressionMultiline(t *testing.T) {
	// Simplified version of StatePerformances.kerml line 77-83
	input := `package Test {
		function allSubstatePerformances {
			in p : Performance;
			return result: StatePerformance[*];
		}
		function includes {
			in seq: Integer[*];
			in elem: Integer;
			return result: Boolean;
		}
		function isEmpty {
			in seq: Integer[*];
			return result: Boolean;
		}
		
		classifier StatePerformance {
			feature accepted: Integer[0..1];
			feature accableT: Integer;
			feature dispatchScope: Performance;
			feature thatSP: StatePerformance;
			feature exit: StatePerformance;
			
			inv { allSubstatePerformances(dispatchScope)->forAll{in oSP : StatePerformance;						
					  oSP == thatSP | isEmpty(oSP.accepted) |
   					  includes(thatSP.exit.successors, oSP.exit) |
					  ( oSP.accepted != accableT & true ) } }
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	t.Logf("Input length: %d bytes", len(input))
	t.Logf("Char at 668: %q", string(input[668:669]))
	t.Logf("Context 660-680: %q", string(input[660:680]))
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

func TestInvariantBodyExpressionSingleLine(t *testing.T) {
	// Single-line version - no newlines
	input := `package Test {
		function allSubstatePerformances { in p : Performance; return result: StatePerformance[*]; }
		function includes { in seq: Integer[*]; in elem: Integer; return result: Boolean; }
		function isEmpty { in seq: Integer[*]; return result: Boolean; }
		
		classifier StatePerformance {
			feature accepted: Integer[0..1];
			feature accableT: Integer;
			feature dispatchScope: Performance;
			feature thatSP: StatePerformance;
			feature exit: StatePerformance;
			
			inv { allSubstatePerformances(dispatchScope)->forAll{in oSP : StatePerformance; oSP == thatSP | isEmpty(oSP.accepted) | includes(thatSP.exit, oSP.exit) | ( oSP.accepted != accableT & true ) } }
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
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

func TestShorthandBodyParameter(t *testing.T) {
	// Test shorthand body param syntax: {name : Type; expr}
	// Used in collection operators like ->exists, ->forAll, ->select
	input := `package Test {
		predicate test {
			in vertices: Point[*];
			vertices->exists{p : Point; p.x > 0}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
	}
}

// Test actual ShapeItems.sysml pattern
func TestShapeItemsExistsPattern(t *testing.T) {
	input := `package Test {
		feature vertices: Point[*];
		predicate isDistinct {
			in p1 : Point;
			vertices->exists{p2 : Point; p1 != p2 and includes(p1.matingOccurrences, p2) }
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
		}
	}
}

func TestBodyParameterWithDocMembers(t *testing.T) {
	// Body param with nested doc comments (TradeStudies.sysml pattern)
	input := `package Test {
		function selectBest {
			in alternatives: Alternative[*];
			return selected = alternatives->selectOne {in ref a {
				doc
				/*
				 * The selected alternative that meets the objective.
				 */
			} objective(a)};
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Error at offset %d: %s", d.Span.Offset, d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
