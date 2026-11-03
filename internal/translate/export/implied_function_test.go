package export

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// impliedFunctionType, which the reader accepts as the function of a call
// reaching an untyped usage, is exactly the Function the semantic model
// implies for a bare usage of each kind — no entry more, none fewer.
func TestImpliedFunctionTypeIsTheModelsImpliedTyping(t *testing.T) {
	keywords := map[string][]string{
		"sysml": {"part", "attribute", "item", "calc", "constraint", "requirement", "concern", "viewpoint", "case", "analysis", "verification", "use case", "action", "state", "view", "rendering", "port", "occurrence", "ref", "enum", "individual", "flow", "connection", "interface", "allocation"},
		"kerml": {"feature", "step", "expr", "bool", "inv", "connector", "succession", "flow"},
	}
	seen := map[string]bool{}
	for lang, kws := range keywords {
		for _, kw := range kws {
			text := fmt.Sprintf("package T {\n\t%s u;\n}\n", kw)
			sf := source.New("t."+lang, []byte(text))
			p := parser.New(sf)
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("%s %s: %v", lang, kw, p.Diagnostics)
			}
			index := libs.NewModelIndex()
			index.AddDocument(sf.Name(), root)
			index.ExpandWildcardImports()
			resolver := resolve.New(index)
			model := semantics.NewModel(resolver)
			resolver.ResolveDocument(sf.Name(), root)
			pkg, _ := index.DocumentRoot(sf.Name()).LookupLocal("T")
			if pkg == nil || pkg.Scope == nil {
				t.Fatalf("%s `%s u;`: no package", lang, kw)
			}
			u, _ := pkg.Scope.LookupLocal("u")
			if u == nil {
				t.Fatalf("%s `%s u;`: no usage named u", lang, kw)
			}
			got := model.FunctionTypes(u)
			metaclass := declaredMetaclass(u.Decl)
			seen[metaclass] = true
			want, listed := impliedFunctionType[metaclass]
			if lang == "kerml" && metaclass == "ConstraintUsage" {
				want = "Performances::BooleanEvaluation" // the reader's `inv` case in typesChainEnd
			}
			if !listed {
				if len(got) != 0 {
					t.Errorf("%s `%s u;` (%s): the model implies %d Function(s), the reader accepts none", lang, kw, metaclass, len(got))
				}
				continue
			}
			expected, ok := resolver.ResolveQualified(nil, ast.QualifiedNameOf(strings.Split(want, "::")...))
			if !ok || expected == nil {
				t.Fatalf("%s does not resolve", want)
			}
			if len(got) != 1 || got[0] != expected {
				names := make([]string, len(got))
				for i, typ := range got {
					names[i] = typ.Name
				}
				t.Errorf("%s `%s u;` (%s): the model implies %v, the reader accepts %s", lang, kw, metaclass, names, want)
			}
		}
	}
	for metaclass := range impliedFunctionType {
		if !seen[metaclass] {
			t.Errorf("impliedFunctionType[%q] is covered by no keyword here", metaclass)
		}
	}
}
