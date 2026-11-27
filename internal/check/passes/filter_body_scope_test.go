package passes

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// filterCodesAt returns the sorted codes of the filter findings of a model whose
// package P holds body, with imp written in place of IMP.
func filterCodesAt(t *testing.T, body, imp string) []string {
	t.Helper()
	const name = "<t>.sysml"
	src := "package Meta { metadata def Safety; }\npackage Q { part def T; }\npackage P { " +
		strings.ReplaceAll(body, "IMP", imp) + " }"
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	var codes []string
	for _, d := range Analyze(name, root, nil, idx) {
		if strings.HasPrefix(d.Code, "filter-") {
			codes = append(codes, d.Code)
		}
	}
	slices.Sort(codes)
	return codes
}

// The filter an import states is checked wherever the import is written. A loop
// body or an if branch has a scope but no symbol, so no member leads to it.
func TestImportFilterIsCheckedInEveryBody(t *testing.T) {
	const action = "action def A { attribute n : ScalarValues::Integer; "
	positions := []struct{ name, body string }{
		{"an action definition body", action + "IMP }"},
		{"a nested action", action + "action inner { IMP } }"},
		{"a then body", action + "if n < 3 { IMP } }"},
		{"an else if body", action + "if n < 3 { } else if n < 5 { IMP } }"},
		{"an else body", action + "if n < 3 { } else { IMP } }"},
		{"a loop body", action + "loop { IMP } }"},
		{"a while body", action + "while n < 3 { IMP } }"},
		{"a for body", action + "for i in (1, 2, 3) { IMP } }"},
		{"a loop in a then body", action + "if n < 3 { loop { IMP } } }"},
		{"a while in an else body", action + "if n < 3 { } else { while n < 2 { IMP } } }"},
		{"a for in a then body", action + "if n < 3 { for i in (1, 2, 3) { IMP } } }"},
		{"a then body in a loop", action + "loop { if n < 3 { IMP } } }"},
		{"a then body in a while", action + "while n < 9 { if n < 3 { IMP } } }"},
		{"an action nested in a then body", action + "if n < 3 { action inner { IMP } } }"},
		{"an action nested in a loop body", action + "loop { action inner { IMP } } }"},
	}
	imports := []struct {
		imp  string
		want []string
	}{
		{"private import Q::*[3];", []string{"filter-not-boolean"}},
		{"private import Q::*[1 + 2];", []string{"filter-not-boolean"}},
		{"private import Q::**[3];", []string{"filter-not-boolean"}},
		{"private import Q::*[@Meta::Safety + 1];", []string{"filter-not-evaluable"}},
		{"private import Q::*[@NoSuchMeta];", nil},
		{"private import Q::*[@Meta::Safety];", nil},
	}
	for _, imp := range imports {
		if got := filterCodesAt(t, "IMP", imp.imp); !slices.Equal(got, imp.want) {
			t.Fatalf("package level, %s: got %v, want %v", imp.imp, got, imp.want)
		}
		for _, pos := range positions {
			t.Run(pos.name+"/"+imp.imp, func(t *testing.T) {
				if got := filterCodesAt(t, pos.body, imp.imp); !slices.Equal(got, imp.want) {
					t.Errorf("got %v, want %v as at package level", got, imp.want)
				}
			})
		}
	}
}
