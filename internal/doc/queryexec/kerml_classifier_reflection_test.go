package queryexec

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// kermlClassifierKernel declares X specializing A with keyword in package K; an association
// or interaction takes its two ends, a `type` the specialization its grammar requires.
func kermlClassifierKernel(keyword string) string {
	body, general := ";", ""
	if strings.HasPrefix(keyword, "assoc") || keyword == "interaction" {
		body = " { end feature a : C; end feature b : C; }"
	}
	if keyword == "type" {
		general = " specializes C"
	}
	return "package K {\n\tclass C;\n\t" + keyword + " A" + general + body + "\n\t" + keyword + " X specializes A" + body + "\n}\n"
}

// Reflection reads each KerML classifier keyword as its metaclass and its `specializes` as a
// Subclassification; `type` is a Type, no Classifier, with a plain Specialization.
func TestKerMLClassifierReflectsItsMetaclassAndSubclassification(t *testing.T) {
	for _, keyword := range ast.KerMLTypeKeywords() {
		metaclass := ast.KerMLTypeMetaclass(keyword)
		relationship := "Subclassification"
		if keyword == "type" {
			relationship = "Specialization"
		}
		t.Run(keyword, func(t *testing.T) {
			fixture := loadExecutionSources(t, "package Observatory {"+executionImports+fmt.Sprintf(`
calc def Declared :> Query {
	in root : Element;
	WhereType(source = Descendants(source = root), type = "%s", exact = true)
}
calc def Specializations :> Query {
	in root : Element;
	Project(source = WhereType(source = Descendants(source = root, relationships = true), type = "%s", exact = true), properties = ("owningRelatedElement"))
}
`, metaclass, relationship)+"}", map[string]string{"k.kerml": kermlClassifierKernel(keyword)})
			root := Bindings{"root": {ElementValue(fixture.qualified(t, "K"))}}
			declared, err := fixture.execute(t, "Declared", root, Options{})
			if err != nil {
				t.Fatalf("Declared: %v", err)
			}
			var got []string
			for _, row := range declared.Rows() {
				got = append(got, symbols.FQNOf(row.Element().Declaration()))
			}
			if !slices.Contains(got, "K::X") {
				t.Errorf("`%s X` is not exactly a %s: the %s elements are %v", keyword, metaclass, metaclass, got)
			}
			specializations, err := fixture.execute(t, "Specializations", root, Options{})
			if err != nil {
				t.Fatalf("Specializations: %v", err)
			}
			if got := rowCells(t, specializations); !slices.Contains(got, "K::X") {
				t.Errorf("the specializes of `%s X` is not exactly a %s: X owns none among %v", keyword, relationship, got)
			}
		})
	}
}
