package symbols

import "testing"

// A requirement's subject, assume and require members are registered under
// their short name as well as their name, as any usage is (`part <p> x`).
func TestBuildRequirementMemberShortNameKeys(t *testing.T) {
	root := build(t, `package P {
	part def T;
	constraint def C;
	requirement def R {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
		subject <t> : T;
		assume constraint <b> : C;
		require constraint <q> : C;
	}
}`)
	pkg, ok := root.LookupLocal("P")
	if !ok || pkg.Scope == nil {
		t.Fatal("P not found")
	}
	req, ok := pkg.Scope.LookupLocal("R")
	if !ok || req.Scope == nil {
		t.Fatal("R not found")
	}
	pairs := []struct {
		short, name string
		kind        SymbolKind
	}{
		{"s", "x", SymbolPartUsage},
		{"a", "ac", SymbolConstraintUsage},
		{"r", "rc", SymbolConstraintUsage},
		{"t", "", SymbolPartUsage},
		{"b", "", SymbolConstraintUsage},
		{"q", "", SymbolConstraintUsage},
	}
	for _, p := range pairs {
		bySort, ok := req.Scope.LookupLocal(p.short)
		if !ok {
			t.Fatalf("short name %q not found in R", p.short)
		}
		if bySort.Kind != p.kind {
			t.Errorf("<%s> kind = %v, want %v", p.short, bySort.Kind, p.kind)
		}
		if bySort.ShortName != p.short {
			t.Errorf("<%s> ShortName = %q, want %q", p.short, bySort.ShortName, p.short)
		}
		if p.name == "" {
			// A short-name-only member is named by its short name, as `part <p>;` is.
			if bySort.Name != p.short {
				t.Errorf("<%s> Name = %q, want the short name", p.short, bySort.Name)
			}
			continue
		}
		byName, ok := req.Scope.LookupLocal(p.name)
		if !ok {
			t.Fatalf("name %q not found in R", p.name)
		}
		if byName != bySort {
			t.Errorf("%q and <%s> map to different symbols", p.name, p.short)
		}
		if byName.Name != p.name {
			t.Errorf("%q Name = %q", p.name, byName.Name)
		}
	}
}

// A redefining subject, assume or require member with a short name of its own
// declares a name (KerML 7.3.4.5), so it takes none from the redefined feature
// and is a member by its short name alone, as `part <p> :>> x` is (the pinned
// pilot reports no duplicate when R2 also declares `subject x`).
func TestBuildRequirementMemberShortNameSuppressesDerivedName(t *testing.T) {
	root := build(t, `package P {
	part def T;
	constraint def C;
	requirement def R {
		subject x : T;
		assume constraint ac : C;
		require constraint rc : C;
	}
	requirement def R2 :> R {
		subject <s> :>> x;
		assume constraint <a> :>> ac;
		require constraint <r> :>> rc;
	}
}`)
	pkg, ok := root.LookupLocal("P")
	if !ok || pkg.Scope == nil {
		t.Fatal("P not found")
	}
	req, ok := pkg.Scope.LookupLocal("R2")
	if !ok || req.Scope == nil {
		t.Fatal("R2 not found")
	}
	for _, p := range []struct{ short, name string }{{"s", "x"}, {"a", "ac"}, {"r", "rc"}} {
		byShort, ok := req.Scope.LookupLocal(p.short)
		if !ok {
			t.Fatalf("short name %q not found in R2", p.short)
		}
		if _, ok := req.Scope.LookupLocal(p.name); ok {
			t.Errorf("R2 answers to %q; a short-named redefinition derives no name", p.name)
		}
		if byShort.Name != p.short || byShort.ShortName != p.short {
			t.Errorf("<%s> = %q <%s>, want a member by <%s> alone", p.short, byShort.Name, byShort.ShortName, p.short)
		}
		if byShort.EffectiveName() || byShort.NamingTarget != nil {
			t.Errorf("<%s> effective=%v target=%v; want a declared name", p.short, byShort.EffectiveName(), byShort.NamingTarget)
		}
	}
}
