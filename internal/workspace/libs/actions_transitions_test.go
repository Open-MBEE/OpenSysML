package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestAcceptedMessageMultipleTyping(t *testing.T) {
	// Exact pattern from Actions.sysml offset 9760
	input := `package Test {
		action a {
			ref acceptedMessage : MessageTransfer, MessageAction :>> trigger {
				in :>> MessageTransfer::payload, MessageAction::payload;
			}
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		t.Logf("  %d. %s", i+1, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}

func TestTransitionWithAcceptVia(t *testing.T) {
	// Pattern from Actions.sysml line 216
	input := `package Test {
		action AcceptAction {
			in receiver: Anything;
			state aState {
				transition aTransition first start accept apayload: Anything via receiver then done;
			}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  offset=%d: %s", d.Span.Offset, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}

func TestNamespaceSuccessionBetweenActions(t *testing.T) {
	// Pattern from Actions.sysml line 479
	input := `package Test {
		action ForEachLoopAction {
			private action initialization
				assign index := 1;
			then private action whileLoop
				while index <= 10 {
					assign var := index;
				}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  offset=%d: %s", d.Span.Offset, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
