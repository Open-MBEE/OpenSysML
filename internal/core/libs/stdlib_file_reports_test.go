package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestActionsDiagnosticsWithContext(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Actions.sysml")
	if err != nil {
		t.Fatalf("Failed to load Actions.sysml: %v", err)
	}

	p := parser.New(source.New("Actions.sysml", data))
	_ = p.ParseFile()

	t.Logf("Actions.sysml: %d diagnostics", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		// Show 60 chars of context around error
		contextStart := byteOffset - 30
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 30
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}

func TestActionsDiagnosticsByOffset(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Systems Library/Actions.sysml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("Actions.sysml", content))
	_ = p.ParseFile()

	t.Logf("Actions.sysml diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(content) {
			char = string(content[d.Span.Offset])
		}
		t.Logf("%d. offset=%d (char=%q): %s", i+1, d.Span.Offset, char, d.Message)
	}
}

func TestItemsDiagnosticsWithContext(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Items.sysml")
	if err != nil {
		t.Fatalf("Failed to load Items.sysml: %v", err)
	}

	p := parser.New(source.New("Items.sysml", data))
	_ = p.ParseFile()

	t.Logf("Items.sysml: %d diagnostics", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		contextStart := byteOffset - 50
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 50
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}

func TestViewsDiagnosticsWithContext(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Views.sysml")
	if err != nil {
		t.Fatalf("Failed to load Views.sysml: %v", err)
	}

	p := parser.New(source.New("Views.sysml", data))
	_ = p.ParseFile()

	t.Logf("Views.sysml: %d diagnostics", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		contextStart := byteOffset - 50
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 50
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}

func TestFeatureReferencingPerformancesDiagnosticsList(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("FeatureReferencingPerformances.kerml", content))
	_ = p.ParseFile()

	t.Logf("FRP diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}
}

func TestTransitionPerformancesDiagnosticsByOffset(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("TransitionPerformances.kerml", content))
	_ = p.ParseFile()

	t.Logf("TransitionPerformances diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(content) {
			char = string(content[d.Span.Offset])
		}
		t.Logf("%d. offset=%d (char=%q): %s", i+1, d.Span.Offset, char, d.Message)
	}
}

func TestOccurrencesDiagnosticsByOffset(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.New(source.New("Occurrences.kerml", data))
	_ = p.ParseFile()

	t.Logf("Occurrences.kerml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}
		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
	}
}

func TestOccurrencesDiagnosticsWithChar(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("Occurrences.kerml", content))
	_ = p.ParseFile()

	t.Logf("Occurrences diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		char := ""
		if d.Span.Offset < len(content) {
			char = string(content[d.Span.Offset])
		}
		t.Logf("%d. offset=%d (char=%q): %s", i+1, d.Span.Offset, char, d.Message)
	}
}

func TestStatePerformancesDiagnosticsList(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to read StatePerformances.kerml: %v", err)
	}

	p := parser.New(source.New("StatePerformances.kerml", data))
	_ = p.ParseFile()

	t.Logf("Total diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		t.Logf("  [%d] Offset %d: %s", i, d.Span.Offset, d.Message)
	}
}

func TestKernelAndAnalysisFileDiagnostics(t *testing.T) {
	src := &embedSource{}

	files := []string{
		"Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml",
		"Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml",
		"Domain Libraries/Analysis/TradeStudies.sysml",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			data, err := src.Read(name)
			if err != nil {
				t.Fatalf("Failed to load %s: %v", name, err)
			}

			file := source.New(name, data)
			p := parser.New(file)
			root := p.ParseFile()

			t.Logf("\n%s diagnostics:", name)
			diags := p.Diagnostics
			for i, d := range diags {
				if i >= 10 {
					break
				}
				t.Logf("  [%d] offset %d: %s", i, d.Span.Offset, d.Message)
				// Show context
				start := d.Span.Offset
				if start < 0 {
					start = 0
				}
				end := start + 80
				if end > len(data) {
					end = len(data)
				}
				t.Logf("      context: %q", data[start:end])
			}

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
		})
	}
}
