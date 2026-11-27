package diag

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// An own-line edit ends in the line ending the document uses, so a fix applied
// to a CRLF document leaves no bare LF behind.
func TestRenderOwnLineKeepsTheDocumentsLineEnding(t *testing.T) {
	crlf := []byte("package P {\r\n    part a;\r\n}\r\n")
	if _, text := InsertLine(17, "import A::*;").Render(crlf); text != "import A::*;\r\n    " {
		t.Errorf("CRLF document: text = %q", text)
	}
	lf := []byte("package P {\n    part a;\n}\n")
	if _, text := InsertLine(16, "import A::*;").Render(lf); text != "import A::*;\n    " {
		t.Errorf("LF document: text = %q", text)
	}
	if _, text := InsertLine(0, "import A::*;").Render(nil); text != "import A::*;\n" {
		t.Errorf("empty document: text = %q", text)
	}
}

// An edit whose span reaches outside the document renders the part inside it, so
// a caller slicing the document by the span stays in bounds.
func TestRenderKeepsTheSpanInsideTheDocument(t *testing.T) {
	content := []byte("package P;\n")
	size := len(content)
	cases := []struct {
		name string
		edit Edit
		want source.Span
	}{
		{"negative offset", Replace(source.Span{Offset: -3, Len: 2}, "x"), source.Span{Offset: 0, Len: 2}},
		{"negative length", Replace(source.Span{Offset: 4, Len: -1}, "x"), source.Span{Offset: 4, Len: 0}},
		{"past the end", Replace(source.Span{Offset: 8, Len: 99}, "x"), source.Span{Offset: 8, Len: size - 8}},
		{"offset past the end", Insert(size+5, "x"), source.Span{Offset: size, Len: 0}},
		{"own line past the end", InsertLine(size+5, "x"), source.Span{Offset: size, Len: 0}},
		{"empty document", Replace(source.Span{Offset: 2, Len: 2}, "x"), source.Span{}},
	}
	for _, c := range cases {
		doc := content
		if c.name == "empty document" {
			doc = nil
		}
		sp, _ := c.edit.Render(doc)
		if sp != c.want {
			t.Errorf("%s: rendered span %+v, want %+v", c.name, sp, c.want)
		}
		_ = doc[sp.Offset : sp.Offset+sp.Len]
	}
}
