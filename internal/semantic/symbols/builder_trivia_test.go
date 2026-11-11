package symbols

import (
	"reflect"
	"strings"
	"testing"
)

// A bare `/* … */` above a declaration is a Comment element of its own, and it
// still reaches the declaration's symbol as leading trivia for hover and completion.
func TestBareCommentsAboveADeclarationAreItsLeadingTrivia(t *testing.T) {
	src := "package P {\n    /* above */\n    part x;\n    /* one */\n    /* two */\n    // note\n    part y;\n}\n"
	pkg, ok := buildScope(t, src).LookupLocal("P")
	if !ok || pkg.Scope == nil {
		t.Fatal("package P was not built")
	}
	want := map[string][]string{
		"x": {"/* above */"},
		"y": {"/* one */", "/* two */", "// note"},
	}
	for name, texts := range want {
		sym, ok := pkg.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("%s was not built", name)
		}
		var got []string
		for _, tr := range sym.LeadingTrivia {
			got = append(got, strings.TrimSpace(src[tr.Span.Offset:tr.Span.End()]))
		}
		if !reflect.DeepEqual(got, texts) {
			t.Errorf("%s leading trivia = %q, want %q", name, got, texts)
		}
	}
}
