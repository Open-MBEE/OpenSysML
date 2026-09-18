package diag

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestRenderPlainEdits(t *testing.T) {
	content := []byte("package P;\n")
	if sp, text := Insert(8, "X").Render(content); sp.Offset != 8 || sp.Len != 0 || text != "X" {
		t.Errorf("Insert rendered as %v %q", sp, text)
	}
	span := source.Span{Offset: 8, Len: 1}
	if sp, text := Replace(span, "Q").Render(content); sp != span || text != "Q" {
		t.Errorf("Replace rendered as %v %q", sp, text)
	}
}

// An own-line edit keeps the indentation of the line it is inserted before, so
// the declaration that line holds is not shifted.
func TestRenderOwnLineKeepsIndentation(t *testing.T) {
	content := []byte("package P {\n\tpart w;\n}\n")
	at := 13 // the member declaration, one tab into its line
	sp, text := InsertLine(at, "import A::*;").Render(content)
	if sp.Offset != at || sp.Len != 0 {
		t.Errorf("span = %v, want an insertion at %d", sp, at)
	}
	if text != "import A::*;\n\t" {
		t.Errorf("text = %q", text)
	}
}

// The indentation is the whitespace between the line start and the insertion
// point, so inserting at the start of an indented line renders no indent.
func TestRenderOwnLineFromLineStart(t *testing.T) {
	content := []byte("package P {\n    part w;\n}\n")
	if _, text := InsertLine(12, "import A::*;").Render(content); text != "import A::*;\n" {
		t.Errorf("insertion at the line start rendered %q", text)
	}
	if _, text := InsertLine(16, "import A::*;").Render(content); text != "import A::*;\n    " {
		t.Errorf("insertion past the indent rendered %q", text)
	}
	// An offset outside the document renders without indentation rather than
	// panicking.
	if _, text := InsertLine(999, "x").Render(content); text != "x\n" {
		t.Errorf("out-of-range insertion rendered %q", text)
	}
}
