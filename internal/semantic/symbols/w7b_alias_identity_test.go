package symbols

import "testing"

// An alias is an element of its own — a Membership naming another element
// (KerML §8.2.3.2) — so each link of a chain keeps its own identity and its own
// name in the member list completion enumerates.
func TestW7BAliasChainKeepsPerLinkIdentity(t *testing.T) {
	scope := buildScope(t, "package test { classifier A; alias A1 for A; alias A2 for A1; }")
	pkg, ok := scope.LookupLocal("test")
	if !ok {
		t.Fatal("package test is not a member of the root scope")
	}
	byName := map[string]*Symbol{}
	for _, m := range pkg.Scope.Members() {
		byName[m.Name] = m
	}
	for _, name := range []string{"A", "A1", "A2"} {
		if byName[name] == nil {
			t.Fatalf("%s is missing from the member list of test", name)
		}
	}
	for _, name := range []string{"A1", "A2"} {
		if byName[name].Kind != SymbolAlias {
			t.Errorf("%s has kind %v, want %v", name, byName[name].Kind, SymbolAlias)
		}
	}
	// Distinct declarations are distinct elements, however the chain is walked.
	pairs := [][2]string{{"A1", "A2"}, {"A", "A1"}, {"A", "A2"}}
	for _, p := range pairs {
		if SameElement(byName[p[0]], byName[p[1]]) {
			t.Errorf("SameElement(%s, %s) = true, want false", p[0], p[1])
		}
	}
	for _, name := range []string{"A", "A1", "A2"} {
		if !SameElement(byName[name], byName[name]) {
			t.Errorf("SameElement(%s, %s) = false", name, name)
		}
	}
}
