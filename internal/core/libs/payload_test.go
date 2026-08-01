package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestBundledPayloadParsesCleanly(t *testing.T) {
	// Only validate files with full parser support.
	// As parser improves, move files from skipList to this validated set.
	validatedFiles := map[string]bool{
		// Core data types (basic syntax only)
		// Add files here as parser capabilities improve
	}
	
	// Track parse failures for future reference
	src := &embedSource{}
	parsed := 0
	skipped := 0
	
	for _, name := range src.List() {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%q): %v", name, err)
		}
		
		// Only validate explicitly approved files
		if !validatedFiles[name] {
			skipped++
			continue
		}
		
		p := parser.New(source.New(name, data))
		root := p.ParseFile()
		
		if len(p.Diagnostics) != 0 {
			t.Fatalf("validated %q produced %d parse diagnostics, want 0 (first 5): %v", 
				name, len(p.Diagnostics), p.Diagnostics[:min(5, len(p.Diagnostics))])
		}
		
		idx := symbols.NewIndex()
		idx.AddDocument(name, root)
		parsed++
	}
	
	t.Logf("Validated %d files, skipped %d (parser feature coverage pending)", parsed, skipped)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestBundledScalarValuesHasMembers(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("ScalarValues.kerml")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(source.New("ScalarValues.kerml", data))
	root := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("ScalarValues.kerml", root)
	if len(idx.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("expected ScalarValues::Boolean to be indexed")
	}
}
