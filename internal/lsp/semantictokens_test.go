package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/highlight"
)

// decoded is one semantic token read back from the protocol's relative encoding.
type decoded struct {
	line, char, length int
	class              string
	modifiers          []string
}

// decodeTokens reverses the relative encoding, so a test asserts on positions
// and legend names rather than on a flat array of numbers.
func decodeTokens(t *testing.T, data []uint32) []decoded {
	t.Helper()
	if len(data)%5 != 0 {
		t.Fatalf("token data length = %d, want a multiple of 5", len(data))
	}
	legend := semanticTokensLegend()
	var out []decoded
	line, char := 0, 0
	for i := 0; i < len(data); i += 5 {
		deltaLine, deltaChar := int(data[i]), int(data[i+1])
		line += deltaLine
		if deltaLine == 0 {
			char += deltaChar
		} else {
			char = deltaChar
		}
		typeIdx := int(data[i+3])
		if typeIdx < 0 || typeIdx >= len(legend.TokenTypes) {
			t.Fatalf("token type index %d outside the legend", typeIdx)
		}
		var mods []string
		for bit, name := range legend.TokenModifiers {
			if data[i+4]&(1<<uint(bit)) != 0 {
				mods = append(mods, string(name))
			}
		}
		out = append(out, decoded{
			line: line, char: char, length: int(data[i+2]),
			class: string(legend.TokenTypes[typeIdx]), modifiers: mods,
		})
	}
	return out
}

// tokenAt returns the token starting at a line and column, or fails.
func tokenAt(t *testing.T, toks []decoded, line, char int) decoded {
	t.Helper()
	for _, tok := range toks {
		if tok.line == line && tok.char == char {
			return tok
		}
	}
	t.Fatalf("no token at %d:%d in %+v", line, char, toks)
	return decoded{}
}

const tokenSource = `package P {
    // a wheel
    part def Wheel {
        attribute pressure;
    }
    part w : Wheel;
    action def Brake {
        in attribute force;
    }
}
`

func TestSemanticTokensFullClassifiesDeclarationsAndReferences(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/tokens.sysml").Filename()
	ws.Open(name, []byte(tokenSource), 1)

	res, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	toks := decodeTokens(t, res.Data)

	// Keyword, comment, declaration and reference all classified, each with the
	// modifiers its declaration justifies.
	for _, want := range []struct {
		line, char, length int
		class              string
		modifiers          string
	}{
		{0, 0, 7, "keyword", ""},                         // package
		{0, 8, 1, "namespace", "declaration"},            // P
		{1, 4, 10, "comment", ""},                        // // a wheel
		{2, 13, 5, "class", "declaration definition"},    // part def Wheel
		{3, 18, 8, "property", "declaration"},            // attribute pressure
		{5, 9, 1, "variable", "declaration"},             // part w
		{5, 13, 5, "class", ""},                          // : Wheel reference
		{6, 15, 5, "function", "declaration definition"}, // action def Brake
		{7, 21, 5, "parameter", "declaration"},           // in attribute force
	} {
		got := tokenAt(t, toks, want.line, want.char)
		if got.length != want.length || got.class != want.class {
			t.Errorf("token at %d:%d = %+v, want length %d class %q",
				want.line, want.char, got, want.length, want.class)
		}
		if mods := strings.Join(got.modifiers, " "); mods != want.modifiers {
			t.Errorf("token at %d:%d modifiers = %q, want %q", want.line, want.char, mods, want.modifiers)
		}
	}
}

// The encoding is relative and ordered: every delta must be non-negative, and a
// token must never overlap the previous one on the same line.
func TestSemanticTokensFullEncodingIsRelativeAndSorted(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/tokens_order.sysml").Filename()
	ws.Open(name, []byte(tokenSource), 1)

	res, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	if len(res.Data) == 0 {
		t.Fatal("no semantic tokens for a document full of declarations")
	}
	line, char := 0, 0
	prevEnd := 0
	for i := 0; i < len(res.Data); i += 5 {
		deltaLine, deltaChar, length := int(res.Data[i]), int(res.Data[i+1]), int(res.Data[i+2])
		if length == 0 {
			t.Errorf("token %d has zero length", i/5)
		}
		if deltaLine == 0 {
			if deltaChar < prevEnd-char {
				t.Errorf("token %d overlaps the previous one on line %d", i/5, line)
			}
			char += deltaChar
		} else {
			line += deltaLine
			char = deltaChar
		}
		prevEnd = char + length
	}
}

// A multi-line comment cannot be one token: the encoding has no way to express a
// token that spans lines, so it is emitted per line.
func TestSemanticTokensFullSplitsMultiLineComment(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/tokens_multiline.sysml").Filename()
	src := "package P {\n    //* two\n     * lines\n     */\n    part w;\n}\n"
	ws.Open(name, []byte(src), 1)

	res, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	comments := 0
	for _, tok := range decodeTokens(t, res.Data) {
		if tok.class == "comment" {
			comments++
			if tok.line < 1 || tok.line > 3 {
				t.Errorf("comment token on line %d, want lines 1..3", tok.line)
			}
		}
	}
	if comments != 3 {
		t.Errorf("comment tokens = %d, want one per line of the note", comments)
	}
}

// Positions and lengths are UTF-16 code units, so a name after an astral-plane
// rune is not shifted by its byte length.
func TestSemanticTokensFullUsesUTF16Units(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/tokens_utf16.sysml").Filename()
	// The note holds one astral-plane rune, two UTF-16 units wide.
	ws.Open(name, []byte("package P {\n    // \U0001F31F\n    part w;\n}\n"), 1)

	res, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	toks := decodeTokens(t, res.Data)
	comment := tokenAt(t, toks, 1, 4)
	if comment.class != "comment" || comment.length != 5 {
		t.Errorf("comment token = %+v, want length 5 in UTF-16 units", comment)
	}
	if part := tokenAt(t, toks, 2, 4); part.class != "keyword" {
		t.Errorf("token after the note = %+v, want the part keyword", part)
	}
}

// A range request answers the tokens of that range only, in the same encoding
// (relative to the first token returned).
func TestSemanticTokensRangeFiltersToTheRange(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/tokens_range.sysml").Filename()
	ws.Open(name, []byte(tokenSource), 1)

	res, err := s.SemanticTokensRange(context.Background(), &protocol.SemanticTokensRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Range: protocol.Range{
			Start: protocol.Position{Line: 5, Character: 0},
			End:   protocol.Position{Line: 6, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("SemanticTokensRange err = %v", err)
	}
	toks := decodeTokens(t, res.Data)
	if len(toks) == 0 {
		t.Fatal("no tokens for a range holding a part usage")
	}
	for _, tok := range toks {
		if tok.line != 5 {
			t.Errorf("token on line %d, want only line 5: %+v", tok.line, toks)
		}
	}
}

// An unknown document has no tokens rather than an error: a client may ask
// before opening.
func TestSemanticTokensFullUnknownDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/tmp/missing.sysml")},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	if len(res.Data) != 0 {
		t.Errorf("data = %v, want none", res.Data)
	}
}

// The legend must list every class and modifier the classifier can emit, in the
// order their indices mean, or an editor colors the wrong tokens.
func TestSemanticTokensLegendCoversTheClassifier(t *testing.T) {
	legend := semanticTokensLegend()
	classes := highlight.Classes()
	if len(legend.TokenTypes) != len(classes) {
		t.Fatalf("legend types = %d, want %d", len(legend.TokenTypes), len(classes))
	}
	for i, class := range classes {
		if got := string(legend.TokenTypes[i]); got != class.String() {
			t.Errorf("legend type %d = %q, want %q", i, got, class)
		}
	}
	mods := highlight.Modifiers()
	if len(legend.TokenModifiers) != len(mods) {
		t.Fatalf("legend modifiers = %d, want %d", len(legend.TokenModifiers), len(mods))
	}
	for i, mod := range mods {
		if got := string(legend.TokenModifiers[i]); got != mod.String() {
			t.Errorf("legend modifier %d = %q, want %q", i, got, mod)
		}
		if mod != 1<<uint(i) {
			t.Errorf("modifier %q has bit %d, want %d", mod, mod, 1<<uint(i))
		}
	}
}

// The cursor answers what a scan from the start of the document answers, for
// offsets in order and out of it, so encoding stays linear without changing.
func TestLineCursorMatchesAScanFromTheStart(t *testing.T) {
	content := []byte("package P {\n    // 🚗 é note\n    part def Wheel;\n\n    part w : Wheel;\n}\n")
	offsets := []int{0, 7, 12, 16, 30, 40, 55, len(content), 3, 0, len(content) - 1}
	cursor := &lineCursor{content: content}
	for _, off := range offsets {
		if got, want := cursor.positionAt(off), offsetToPosition(content, off); got != want {
			t.Errorf("positionAt(%d) = %v, want %v", off, got, want)
		}
	}
}
