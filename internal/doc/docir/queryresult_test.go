package docir

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

func TestQueryResultRefusesNilRendering(t *testing.T) {
	document, err := QueryResult(nil)
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("QueryResult(nil) = %v, %v; want an ErrorMissingRendering", document, err)
	}
	if document != nil {
		t.Fatalf("QueryResult(nil) returned a document beside its error: %v", document)
	}
}

func TestQueryResultOfZeroRendering(t *testing.T) {
	document, err := QueryResult(&view.Rendering{})
	if err != nil {
		t.Fatalf("QueryResult of a zero rendering: %v", err)
	}
	if len(document.content) != 2 || document.content[1].kind != ContentParagraph {
		t.Fatalf("a rendering of no rows is its header and a paragraph, got %+v", document.content)
	}
}
