package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestClassifierAndBaseLibraryPattern(t *testing.T) {
	// Test 1: classifier keyword
	code1 := `package Test {
		abstract classifier Anything {
		}
	}`

	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	root1 := p1.ParseFile()
	if len(p1.Diagnostics) > 0 {
		t.Errorf("Test 1 failed with %d diagnostics:", len(p1.Diagnostics))
		for _, d := range p1.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}

	// Check AST
	if root1 == nil {
		t.Fatal("root1 is nil")
	}
	if len(root1.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(root1.Members))
	}

	// Test 2: Base.kerml minimal pattern
	code2 := `standard library package Base {
		doc 
		/*
		 * This package defines the classifiers.
		 */
		
		abstract classifier Anything {
			doc
			/*
			 * Anything is the top level.
			 */
			
			feature self: Anything[1];
		}
	}`

	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	if len(p2.Diagnostics) > 0 {
		t.Errorf("Test 2 (Base.kerml pattern) failed with %d diagnostics:", len(p2.Diagnostics))
		for _, d := range p2.Diagnostics {
			t.Errorf("  - offset=%d, %s", d.Span.Offset, d.Message)
		}
	}
}
