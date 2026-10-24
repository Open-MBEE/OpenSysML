package semantics

import "testing"

// The base feature a declaration's kind implies, and whether its declared chain
// already reaches it, are looked up once per symbol: a repeated query allocates
// nothing and answers as the first did.
func TestKindBaseIsLookedUpOncePerSymbol(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Vehicle;
		part def Car :> Vehicle;
		part car : Car;
		part wheel :> Parts::parts;
	}`)
	p := sym(t, root, "P")
	car := nested(t, p.Scope, "car")
	wheel := nested(t, p.Scope, "wheel")
	base := m.implicitUsageBaseFeature(car)
	if base == nil || base.Name != "parts" {
		t.Fatalf("implicitUsageBaseFeature(car) = %v, want Parts::parts", base)
	}
	if got := m.implicitUsageBaseFeature(wheel); got != nil {
		t.Fatalf("implicitUsageBaseFeature(wheel) = %v, want nil: its declared chain reaches Parts::parts", got)
	}
	if !m.declaredChainReachesKindBase(wheel) || m.declaredChainReachesKindBase(car) {
		t.Fatalf("declaredChainReachesKindBase: wheel %v, car %v, want true, false", m.declaredChainReachesKindBase(wheel), m.declaredChainReachesKindBase(car))
	}
	for name, query := range map[string]func(){
		"implicitUsageBaseFeature":     func() { m.implicitUsageBaseFeature(car) },
		"implicitBaseUsage":            func() { m.implicitBaseUsage(car) },
		"declaredChainReachesKindBase": func() { m.declaredChainReachesKindBase(wheel) },
		"ImplicitGenerals":             func() { m.ImplicitGenerals(car) },
	} {
		if got := testing.AllocsPerRun(20, query); got != 0 {
			t.Errorf("a repeated %s query allocates %.0f times, want 0", name, got)
		}
	}
	if got := m.implicitUsageBaseFeature(car); got != base {
		t.Fatalf("memoized implicitUsageBaseFeature(car) = %v, want %v", got, base)
	}
}
