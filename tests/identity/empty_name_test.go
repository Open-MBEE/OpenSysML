package identity_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The empty unrestricted name is a name (KerML §8.2.2.3), so its element
// derives its id from that name; only the unnamed member beside it is positional.
func TestEmptyNamedElementDerivesItsIDFromItsName(t *testing.T) {
	table, idx := buildTable(t, `package P {
	part def Pt;
	part '' : Pt;
	part '   ' : Pt;
	part : Pt;
}
`)
	owner := idx.LookupQualified("P")
	if len(owner) != 1 {
		t.Fatalf("P: %d symbols", len(owner))
	}
	// The id encodes identity segments: the empty name quoted, a blank one as it is.
	for _, c := range []struct{ name, qualified string }{
		{"", "P::''"},
		{"   ", "P::   "},
	} {
		matches := owner[0].Scope.LookupLocalAll(c.name)
		if len(matches) != 1 {
			t.Fatalf("member %q of P: %d symbols, want 1", c.name, len(matches))
		}
		sym := matches[0]
		if !sym.Named() || sym.Name != c.name {
			t.Errorf("member %q: Named() = %v, Name = %q", c.name, sym.Named(), sym.Name)
		}
		info, ok := table.Info(sym)
		if !ok {
			t.Fatalf("member %q: no identity info", c.name)
		}
		if want := rdf.EncodeElementID(c.qualified); info.EffectiveID != want {
			t.Errorf("member %q: id %q, want %q derived from %s", c.name, info.EffectiveID, want, c.qualified)
		}
	}
	anonymous := 0
	owner[0].Scope.ForEachAnonymousMember(func(sym *symbols.Symbol) bool {
		anonymous++
		if sym.Named() {
			t.Errorf("anonymous member answers Named(): %q", sym.Name)
		}
		return true
	})
	if anonymous != 1 {
		t.Errorf("P has %d anonymous members, want the one unnamed part", anonymous)
	}
}
