package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const modifierLibrary = `standard library package Lib {
	abstract part def Vehicle;
	individual part def Fleet1 :> Vehicle;
	part def Wheel;
	individual part car1 : Fleet1;
	part wheel : Wheel;
	attribute def Mass;
}
`

// The declaration-borne query properties of a library element read the same
// whether its symbol was parsed, restored from the on-disk cache or decoded
// from a snapshot: every path carries the declaration.
func TestLibraryQueryPropertiesSurviveEveryLoadPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(modifierLibrary), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()

	src := NewDirSource(dir)
	data, err := BuildSnapshot(src)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	snapshot, err := DecodeSnapshot(data, NewLoader(src, nil).setDigest())
	if err != nil {
		t.Fatalf("DecodeSnapshot: %v", err)
	}
	paths := map[string]*symbols.Index{
		"parsed":   loadWholeLibrary(t, dir, cacheDir),
		"restored": loadWholeLibrary(t, dir, cacheDir),
		"snapshot": snapshot,
	}

	want := map[string]map[string]string{
		"Lib::Vehicle": {query.PropertyIsAbstract: "true", query.PropertyIsIndividual: "false"},
		"Lib::Fleet1":  {query.PropertyIsAbstract: "false", query.PropertyIsIndividual: "true"},
		"Lib::Wheel":   {query.PropertyIsAbstract: "false", query.PropertyIsIndividual: "false"},
		"Lib::car1":    {query.PropertyIsAbstract: "false", query.PropertyIsIndividual: "true"},
		"Lib::wheel":   {query.PropertyIsAbstract: "false", query.PropertyIsIndividual: "false"},
		"Lib::Mass":    {query.PropertyIsAbstract: "false", query.PropertyIsIndividual: "false"},
	}
	for path, idx := range paths {
		r := resolve.New(idx)
		reader := query.NewPropertyReader(idx, r, semantics.NewModel(r))
		for fqn, props := range want {
			matches := symbols.PreferDeclared(idx.LookupQualified(fqn))
			if len(matches) != 1 {
				t.Fatalf("%s: %s matched %d symbols, want 1", path, fqn, len(matches))
			}
			if matches[0].Decl == nil {
				t.Errorf("%s: %s carries no declaration", path, fqn)
			}
			for prop, value := range props {
				got, ok := reader.Values(matches[0], prop)
				if !ok || len(got) != 1 || got[0] != value {
					t.Errorf("%s: %s.%s = %v, %v; want [%s], true", path, fqn, prop, got, ok, value)
				}
			}
		}
		if _, ok := reader.Values(symbols.PreferDeclared(idx.LookupQualified("Lib"))[0], query.PropertyIsIndividual); ok {
			t.Errorf("%s: the package Lib has an isIndividual value, want none", path)
		}
	}
}
