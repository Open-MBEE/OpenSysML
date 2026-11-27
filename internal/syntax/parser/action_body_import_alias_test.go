package parser

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A loop or branch body is an ActionBodyParameter taking ActionBodyItem* (SysML.xtext), so it
// admits the Import and AliasMember of a NonBehaviorBodyItem as a plain action body does.
func TestLoopAndBranchBodiesAdmitImportAndAlias(t *testing.T) {
	bodies := map[string]string{
		"action": "action a { %s }",
		"while":  "while (n < 3) { %s }",
		"loop":   "loop { %s } until n > 3;",
		"for":    "for q in (1, 2) { %s }",
		"if":     "if n < 3 { %s }",
		"else":   "if n < 3 { } else { %s }",
	}
	members := map[string]string{
		"private import": "private import Q::*;",
		"public import":  "public import Q::*;",
		"private alias":  "private alias TT for Q::T;",
		"alias":          "alias TT for Q::T;",
		// Unprefixed, the import is still an import: only its missing visibility is reported.
		"import": "import Q::*;",
	}
	for bodyName, body := range bodies {
		for memberName, member := range members {
			t.Run(bodyName+"/"+memberName, func(t *testing.T) {
				src := "package Q { part def T; }\npackage P { action def A { attribute n : ScalarValues::Integer; " +
					fmt.Sprintf(body, member+" part v : Q::T;") + " } }"
				p := New(source.New("p.sysml", []byte(src)))
				file := p.ParseFile()
				for _, d := range p.Diagnostics {
					if memberName == "import" && d.Code == "import-visibility" {
						continue
					}
					t.Errorf("unexpected diagnostic: %s", d.Message)
				}
				imports, aliases := 0, 0
				ast.Inspect(file, func(n ast.Node) bool {
					switch n.(type) {
					case *ast.Import:
						imports++
					case *ast.Alias:
						aliases++
					}
					return true
				})
				wantImports, wantAliases := 1, 0
				if memberName == "alias" || memberName == "private alias" {
					wantImports, wantAliases = 0, 1
				}
				if imports != wantImports || aliases != wantAliases {
					t.Errorf("parsed %d imports and %d aliases, want %d and %d", imports, aliases, wantImports, wantAliases)
				}
			})
		}
	}
}
