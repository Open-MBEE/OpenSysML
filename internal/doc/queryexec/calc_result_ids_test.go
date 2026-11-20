package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A calc or verification body's closing expression reads the ids the conversion mints for
// the member stating it, and its operands and their parts read the ids minted under that.
func TestCalcResultExpressionReadsTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    private import ScalarValues::*;
    calc def Twice { in x : Real; x * 2.0 }
    calc def PassIf { in ok : Boolean; return r : Boolean = ok; }
    verification def Fits { subject mass : Real; PassIf(mass <= 30.0) }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	for def, want := range map[string]string{
		"Twice": "Observatory__Twice__x Observatory__Twice___401",
		"Fits":  "Observatory__Fits__mass Observatory__Fits___401",
	} {
		sym := fixture.symbol(t, def)
		members, _ := fixture.model.ReflectiveElements(sym, "ownedMember")
		if got := strings.Join(elementIDs(elementID, members), " "); got != want {
			t.Fatalf("%s ownedMember ids = %q, want %q", def, got, want)
		}
		result := members[1]
		parameters, _ := fixture.model.ReflectiveElements(result, "ownedFeature")
		prefix := "Observatory__" + def + "___401"
		wantParts := prefix + "_pin0 " + prefix + "_pin1 " + prefix + "_pout"
		if def == "Fits" {
			wantParts = prefix + "_pin0 " + prefix + "_pout"
		}
		if got := strings.Join(elementIDs(elementID, parameters), " "); got != wantParts {
			t.Fatalf("%s result ownedFeature ids = %q, want %q", def, got, wantParts)
		}
		var walk func(sym *symbols.Symbol, depth int)
		walk = func(sym *symbols.Symbol, depth int) {
			owned, _ := fixture.model.ReflectiveElements(sym, "ownedElement")
			for _, child := range owned {
				if elementID(child) == "" {
					t.Errorf("a %s under %s's result reads no id", fixture.model.MetaclassOf(child).Name, def)
				}
				if depth < 5 {
					walk(child, depth+1)
				}
			}
		}
		walk(result, 0)
	}
}
