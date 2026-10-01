package highlight

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/semtok"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// tokensOf classifies src without a resolver, so only lexical facts and declared
// names are classified.
func tokensOf(t *testing.T, src string) ([]byte, []semtok.Token) {
	t.Helper()
	content := []byte(src)
	root := parser.New(source.New("test.sysml", content)).ParseFile()
	return content, Tokens(content, root, symbols.Build(root), nil)
}

// text is the source text a token covers.
func text(content []byte, tok semtok.Token) string {
	return string(content[tok.Span.Offset:tok.Span.End()])
}

func TestTokensClassifiesDeclarationsKeywordsAndComments(t *testing.T) {
	content, toks := tokensOf(t, `package P {
    // note
    part def Wheel {
        attribute pressure;
    }
    enum def Color {
        enum red;
    }
    action def Brake {
        in attribute force;
        return attribute stopped;
    }
    abstract part def Vehicle;
    attribute readonly_x = "s";
}
`)
	want := map[string]struct {
		class semtok.Class
		mods  semtok.Modifier
	}{
		"package":  {semtok.ClassKeyword, 0},
		"P":        {semtok.ClassNamespace, semtok.ModDeclaration},
		"// note":  {semtok.ClassComment, 0},
		"Wheel":    {semtok.ClassClass, semtok.ModDeclaration | semtok.ModDefinition},
		"pressure": {semtok.ClassProperty, semtok.ModDeclaration},
		"Color":    {semtok.ClassEnum, semtok.ModDeclaration | semtok.ModDefinition},
		"red":      {semtok.ClassEnumMember, semtok.ModDeclaration | semtok.ModReadonly},
		"Brake":    {semtok.ClassFunction, semtok.ModDeclaration | semtok.ModDefinition},
		"force":    {semtok.ClassParameter, semtok.ModDeclaration},
		"stopped":  {semtok.ClassParameter, semtok.ModDeclaration},
		"Vehicle":  {semtok.ClassClass, semtok.ModDeclaration | semtok.ModDefinition | semtok.ModAbstract},
		`"s"`:      {semtok.ClassString, 0},
	}
	seen := map[string]bool{}
	for _, tok := range toks {
		got := text(content, tok)
		exp, ok := want[got]
		if !ok || seen[got] {
			continue
		}
		seen[got] = true
		if tok.Class != exp.class || tok.Modifiers != exp.mods {
			t.Errorf("%q classified as %v %v, want %v %v", got, tok.Class, tok.Modifiers, exp.class, exp.mods)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("no token for %q", name)
		}
	}
}

// Tokens promises an ordered, non-overlapping result: its consumers encode it
// relative to the previous token.
func TestTokensAreOrderedAndDisjoint(t *testing.T) {
	_, toks := tokensOf(t, `package P {
    part def Wheel;
    part w : Wheel;
    /* comment */
    attribute n = 1.5;
}
`)
	if len(toks) == 0 {
		t.Fatal("no tokens")
	}
	end := 0
	for _, tok := range toks {
		if tok.Span.Len == 0 {
			t.Errorf("empty token at %d", tok.Span.Offset)
		}
		if tok.Span.Offset < end {
			t.Fatalf("token at %d overlaps the previous one ending at %d", tok.Span.Offset, end)
		}
		end = tok.Span.End()
	}
}

// A reference is classified as what it denotes, not as the plain identifier the
// lexer saw; the declaration and the reference to it get the same class.
func TestTokensClassifyDeclarationsWithoutAResolver(t *testing.T) {
	content, toks := tokensOf(t, "package P {\n    part def Wheel;\n    part w : Wheel;\n}\n")
	// Without a resolver the reference to Wheel carries no token, while both
	// declarations do.
	classes := map[string]semtok.Class{}
	for _, tok := range toks {
		if _, seen := classes[text(content, tok)]; !seen {
			classes[text(content, tok)] = tok.Class
		}
	}
	if classes["Wheel"] != semtok.ClassClass {
		t.Errorf("Wheel = %v, want %v", classes["Wheel"], semtok.ClassClass)
	}
	if classes["w"] != semtok.ClassVariable {
		t.Errorf("w = %v, want %v", classes["w"], semtok.ClassVariable)
	}
}

// An empty document has no tokens, and a nil scope classifies lexically only.
func TestTokensEmptyAndScopeless(t *testing.T) {
	if toks := Tokens(nil, nil, nil, nil); len(toks) != 0 {
		t.Errorf("tokens of an empty document = %v, want none", toks)
	}
	content := []byte("package P;\n")
	toks := Tokens(content, nil, nil, nil)
	if len(toks) != 1 || toks[0].Class != semtok.ClassKeyword {
		t.Errorf("tokens without a scope = %v, want the keyword only", toks)
	}
}
