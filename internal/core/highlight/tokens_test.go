package highlight

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// tokensOf classifies src without a resolver, so only lexical facts and declared
// names are classified.
func tokensOf(t *testing.T, src string) ([]byte, []Token) {
	t.Helper()
	content := []byte(src)
	root := parser.New(source.New("test.sysml", content)).ParseFile()
	return content, Tokens(content, root, symbols.Build(root), nil)
}

// text is the source text a token covers.
func text(content []byte, tok Token) string {
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
		class Class
		mods  Modifier
	}{
		"package":  {ClassKeyword, 0},
		"P":        {ClassNamespace, ModDeclaration},
		"// note":  {ClassComment, 0},
		"Wheel":    {ClassClass, ModDeclaration | ModDefinition},
		"pressure": {ClassProperty, ModDeclaration},
		"Color":    {ClassEnum, ModDeclaration | ModDefinition},
		"red":      {ClassEnumMember, ModDeclaration | ModReadonly},
		"Brake":    {ClassFunction, ModDeclaration | ModDefinition},
		"force":    {ClassParameter, ModDeclaration},
		"stopped":  {ClassParameter, ModDeclaration},
		"Vehicle":  {ClassClass, ModDeclaration | ModDefinition | ModAbstract},
		`"s"`:      {ClassString, 0},
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
	classes := map[string]Class{}
	for _, tok := range toks {
		if _, seen := classes[text(content, tok)]; !seen {
			classes[text(content, tok)] = tok.Class
		}
	}
	if classes["Wheel"] != ClassClass {
		t.Errorf("Wheel = %v, want %v", classes["Wheel"], ClassClass)
	}
	if classes["w"] != ClassVariable {
		t.Errorf("w = %v, want %v", classes["w"], ClassVariable)
	}
}

// An empty document has no tokens, and a nil scope classifies lexically only.
func TestTokensEmptyAndScopeless(t *testing.T) {
	if toks := Tokens(nil, nil, nil, nil); len(toks) != 0 {
		t.Errorf("tokens of an empty document = %v, want none", toks)
	}
	content := []byte("package P;\n")
	toks := Tokens(content, nil, nil, nil)
	if len(toks) != 1 || toks[0].Class != ClassKeyword {
		t.Errorf("tokens without a scope = %v, want the keyword only", toks)
	}
}

// Every class and modifier the legend exposes must name itself, since the names
// are what an editor is told.
func TestClassAndModifierNames(t *testing.T) {
	for i, class := range Classes() {
		if class.String() == "unknown" || class.String() == "" {
			t.Errorf("class %d has no name", i)
		}
	}
	for i, mod := range Modifiers() {
		if mod.String() == "unknown" {
			t.Errorf("modifier %d has no name", i)
		}
	}
	if got := Class(-1).String(); got != "unknown" {
		t.Errorf("Class(-1) = %q", got)
	}
	if got := Modifier(1 << 20).String(); got != "unknown" {
		t.Errorf("unknown modifier bit = %q", got)
	}
}
