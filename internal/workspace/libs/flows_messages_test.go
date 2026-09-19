package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestAbstractMessageFeature(t *testing.T) {
	input := `package Test {
		abstract message messages: Message[0..*] nonunique {
			doc
			/* messages is the base feature of all FlowUsages. */
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}

func TestEventOccurrenceParameters(t *testing.T) {
	input := `package Test {
		flow def Message {
			in event occurrence sourceEvent [1] default self.start {
				doc
				/* start */
			}
			in event occurrence targetEvent [1] default self.done {
				doc
				/* end */
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}

func TestRefWithBody(t *testing.T) {
	input := `package Test {
		flow def Message {
			ref payload [0..*] {
				doc
				/* payload */
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			byteOffset := d.Span.Offset
			char := ""
			inputBytes := []byte(input)
			if byteOffset < len(inputBytes) {
				char = string(inputBytes[byteOffset])
			}
			t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
