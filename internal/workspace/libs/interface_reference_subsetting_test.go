package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The reference form of satisfy and verify reads as a ReferenceSubsetting from the
// recorded interface as it does from the parse; a plain `:>` stays a Subsetting.
func TestRecordedReferenceFormSubsettingKeepsItsMetaclass(t *testing.T) {
	const name = "req.sysml"
	const text = `package P {
		part def Sys;
		requirement def R;
		requirement r : R;
		part sys : Sys;
		part x : Sys {
			satisfy r by sys;
			verify r;
			part y :> sys;
		}
		verification def V { objective { verify r; } }
	}`
	idx := NewModelIndex()
	root := parser.New(source.New(name, []byte(text))).ParseFile()
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(name, root)
	parsed := subsettingMetaclasses(t, model, idx.DocumentRoot(name))

	rec, err := WriteInterface(name, source.KindSysML, idx, r, model, nil, nil)
	if err != nil {
		t.Fatalf("WriteInterface: %v", err)
	}
	recorded, err := symbols.BuildRecorded(rec.Scope, name)
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	ridx := NewModelIndex()
	ridx.AddRecordedDocument(name, source.KindSysML, recorded, nil)
	rr := resolve.New(ridx)
	rmodel := semantics.NewModel(rr)
	rr.SetModel(rmodel)
	got := subsettingMetaclasses(t, rmodel, ridx.DocumentRoot(name))

	want := map[string]string{"satisfy": "ReferenceSubsetting", "verify": "ReferenceSubsetting",
		"objective verify": "ReferenceSubsetting", "y": "Subsetting"}
	for owner, metaclass := range want {
		if parsed[owner] != metaclass {
			t.Errorf("%s parsed: %q, want %q", owner, parsed[owner], metaclass)
		}
		if got[owner] != metaclass {
			t.Errorf("%s recorded: %q, want %q", owner, got[owner], metaclass)
		}
	}
}

// subsettingMetaclasses is the metaclass of the `subsets` relationship each
// satisfy, verify, objective verify and `y` of the fixture owns.
func subsettingMetaclasses(t *testing.T, model *semantics.Model, root *symbols.Scope) map[string]string {
	t.Helper()
	out := map[string]string{}
	var visit func(scope *symbols.Scope, inObjective bool)
	visit = func(scope *symbols.Scope, inObjective bool) {
		for _, sym := range scope.AllMembers() {
			kind, isUsage := sym.UsageKind()
			var owner string
			switch {
			case isUsage && kind == ast.UsageSatisfy && sym.Keyword() == "verify" && inObjective:
				owner = "objective verify"
			case isUsage && kind == ast.UsageSatisfy:
				owner = sym.Keyword()
			case sym.Name == "y":
				owner = "y"
			}
			if owner != "" {
				for _, rel := range model.ImplicitRelationships(sym) {
					if rel.Implicit != nil && rel.Implicit.Kind == ast.RelSubsets {
						out[owner] = model.MetaclassOf(rel).Name
					}
				}
			}
			if sym.Scope != nil {
				visit(sym.Scope, inObjective || (isUsage && kind == ast.UsageObjective))
			}
		}
	}
	visit(root, false)
	if len(out) != 4 {
		t.Fatalf("found %d subsetting owners (%v), want 4", len(out), out)
	}
	return out
}
