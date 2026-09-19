package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `use case` member of a definition body keeps both keywords: `use` is not a
// kind prefix qualifying `case`, so the usage is a use case, not a case.
func TestParseUseCaseInDefinitionBody(t *testing.T) {
	for _, src := range []string{
		"package p { part def B { use case uc : B; } }",
		"package p { part def B { use case uc; } }",
		"package p { part def B { use case uc { } } }",
		"package p { action def A { use case uc : A; } }",
	} {
		root := New(source.New("t.sysml", []byte(src))).ParseFile()
		dump := ast.Dump(root)
		if strings.Contains(dump, `(Usage kind="case" name="uc"`) {
			t.Fatalf("`use` dropped in %q:\n%s", src, dump)
		}
		if !strings.Contains(dump, `(Usage kind="use case" name="uc"`) {
			t.Fatalf("no use case usage in %q:\n%s", src, dump)
		}
	}
}

// A `use case def` member of a body is still a use case definition.
func TestParseUseCaseDefInDefinitionBody(t *testing.T) {
	src := "package p { part def B { use case def UC; } }"
	dump := ast.Dump(New(source.New("t.sysml", []byte(src))).ParseFile())
	if !strings.Contains(dump, `(Definition kind="use case" abstract=false variation=false name="UC"`) {
		t.Fatalf("no use case definition:\n%s", dump)
	}
}

// A keyword that does prefix a kind is still consumed as a prefix.
func TestParseKindPrefixStillPrefixesInBody(t *testing.T) {
	src := "package p { part def B { assert constraint c { true } } }"
	dump := ast.Dump(New(source.New("t.sysml", []byte(src))).ParseFile())
	if !strings.Contains(dump, `(Usage kind="constraint" name="c"`) {
		t.Fatalf("prefix keyword not applied:\n%s", dump)
	}
}

// Malformed `use` members produce diagnostics without panicking.
func TestParseUseCaseMalformedInBody(t *testing.T) {
	for _, src := range []string{
		"package p { part def B { use } }",
		"package p { part def B { use case } }",
		"package p { part def B { use case : } }",
	} {
		p := New(source.New("t.sysml", []byte(src)))
		if p.ParseFile() == nil {
			t.Fatalf("nil tree for %q", src)
		}
		if len(p.Diagnostics) == 0 {
			t.Fatalf("no diagnostic for %q", src)
		}
	}
}
