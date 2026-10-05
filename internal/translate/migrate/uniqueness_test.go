package migrate

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// implicitlyUnique agrees with the checker: for every usage keyword the migrator
// writes in every owner it writes it in, the usage may be declared nonunique
// exactly when the checker accepts it so (subsetting-uniqueness-conformance).
func TestImplicitlyUniqueAgreesWithTheChecker(t *testing.T) {
	checked := 0
	for owner := range nestedOwners {
		for kw := range usageKinds {
			for _, prefix := range []string{"", "ref "} {
				usage := prefix + kw + " x[*]"
				if errs := validate(owner.keyword() + " O { " + usage + "; }"); len(errs) > 0 {
					continue // not a usage the owner admits; nonunique is not the question
				}
				errs := validate(owner.keyword() + " O { " + usage + " nonunique; }")
				unique := implicitlyUnique(kw, prefix, "", owner)
				switch {
				case len(errs) > 0 && unique == "":
					t.Errorf("%s in %s: the checker refuses nonunique (%v) but the migrator writes it", usage, owner.keyword(), errs)
				case len(errs) == 0 && unique != "":
					t.Errorf("%s in %s: the checker accepts nonunique but the migrator drops it for %s", usage, owner.keyword(), unique)
				}
				checked++
			}
		}
	}
	if checked < 20 {
		t.Errorf("only %d cases checked", checked)
	}
	for _, c := range []struct{ kw, prefix, dir string }{{"part", "", "in "}, {"part", "ref ", ""}, {"attribute", "", ""}} {
		if got := implicitlyUnique(c.kw, c.prefix, c.dir, catPartDef); got != "" {
			t.Errorf("%s%s%s in a part def: %q, want nothing", c.dir, c.prefix, c.kw, got)
		}
	}
	if !libraryUnique("Items::Item::subparts") || libraryUnique("Base::dataValues") || libraryUnique("Nowhere::nothing") {
		t.Error("libraryUnique misreads the library")
	}
}

func validate(body string) []string {
	ws := model.NewWorkspace()
	ws.Open("t.sysml", []byte("package P { "+body+" }"), 1)
	var errs []string
	for _, d := range ws.Diagnostics("t.sysml") {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return []string{strings.Join(errs, "; ")}
}
