package queryexec

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The expressions a statement of an action body writes at its own slots — an assignment's target and
// value, a loop's conditions and collection, an `if` condition, a `terminate` target — read the ids the
// conversion mints under the statement, each once, on the node written as the statement too.
func TestStatementOperandsReadTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    part def PD { attribute w; }
    part p : PD;
    action def T;
    action def Assign { assign p.w := 1; }
    action def NamedAssign { action n assign p.w := 1; }
    action def TypedAssign { action n : T assign p.w := 1; }
    action def For { for i in 1..3 { } }
    action def NamedFor { action n for i in 1..3 { } }
    action def While { while 1 < 2 { } until 2 < 1; }
    action def Loop { loop { } until 2 < 1; }
    action def If { if 1 < 2 { } }
    action def Terminate { action m; terminate m; }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	for _, tc := range []struct {
		definition string
		want       []string
	}{
		{"Assign", []string{"Observatory__Assign___400_ptarget", "Observatory__Assign___400_ptarget_pa0", "Observatory__Assign___400_pvalueParameter_pvalue"}},
		{"NamedAssign", []string{"Observatory__NamedAssign__n_ptarget", "Observatory__NamedAssign__n_ptarget_pa0", "Observatory__NamedAssign__n_pvalueParameter_pvalue"}},
		{"TypedAssign", []string{"Observatory__TypedAssign__n_ptarget", "Observatory__TypedAssign__n_ptarget_pa0", "Observatory__TypedAssign__n_pvalueParameter_pvalue"}},
		{"For", []string{"Observatory__For___400_pcollectionParameter_pvalue", "Observatory__For___400_pcollectionParameter_pvalue_pa0", "Observatory__For___400_pcollectionParameter_pvalue_pa1"}},
		{"NamedFor", []string{"Observatory__NamedFor__n_pcollectionParameter_pvalue", "Observatory__NamedFor__n_pcollectionParameter_pvalue_pa0", "Observatory__NamedFor__n_pcollectionParameter_pvalue_pa1"}},
		{"While", []string{
			"Observatory__While___400_puntilCondition", "Observatory__While___400_puntilCondition_pa0", "Observatory__While___400_puntilCondition_pa1",
			"Observatory__While___400_pwhileCondition", "Observatory__While___400_pwhileCondition_pa0", "Observatory__While___400_pwhileCondition_pa1",
		}},
		{"Loop", []string{"Observatory__Loop___400_puntilCondition", "Observatory__Loop___400_puntilCondition_pa0", "Observatory__Loop___400_puntilCondition_pa1"}},
		{"If", []string{"Observatory__If___400_pcondition", "Observatory__If___400_pcondition_pa0", "Observatory__If___400_pcondition_pa1"}},
		{"Terminate", []string{"Observatory__Terminate___401_pexpression"}},
	} {
		var expressions []string
		seen := map[*symbols.Symbol]bool{}
		var walk func(sym *symbols.Symbol)
		walk = func(sym *symbols.Symbol) {
			owned, _ := fixture.model.ReflectiveElements(sym, "ownedRelationship")
			for _, rel := range owned {
				if rel.Implicit == nil || rel.Implicit.Member == nil || seen[rel.Implicit.Member] {
					continue
				}
				member := rel.Implicit.Member
				seen[member] = true
				id := elementID(member)
				if id == "" {
					t.Errorf("%s: a %s (%s) reads no id", tc.definition, fixture.model.MetaclassOf(member).Name, member.Name)
				}
				if strings.HasSuffix(fixture.model.MetaclassOf(member).Name, "Expression") || strings.HasPrefix(fixture.model.MetaclassOf(member).Name, "Literal") {
					expressions = append(expressions, id)
				}
				walk(member)
			}
		}
		walk(fixture.symbol(t, tc.definition))
		slices.Sort(expressions)
		if got, want := strings.Join(expressions, " "), strings.Join(tc.want, " "); got != want {
			t.Errorf("%s: expression ids =\n  %s\nwant\n  %s", tc.definition, got, want)
		}
	}
}
