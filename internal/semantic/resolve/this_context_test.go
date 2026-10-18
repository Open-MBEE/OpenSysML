package resolve

import "testing"

// A use case usage is an action usage: `this` written in its body, as in an
// action's, denotes the object owning the performance, not the usage itself.
func TestThisContextWalksThroughACaseUsage(t *testing.T) {
	idx := indexOf(t, map[string]string{"this.sysml": `package test {
		part def Host {
			attribute live;
			use case observe {
				first start;
				then action read { assign live := this.live; }
				then done;
			}
			analysis case measure { action probe; }
		}
		use case standalone { action step; }
	}`})
	r := New(idx)
	pkg, ok := idx.DocumentRoot("this.sysml").LookupLocal("test")
	if !ok {
		t.Fatal("package test not found")
	}
	host, _ := pkg.Scope.LookupLocal("Host")
	for _, path := range [][]string{{"Host", "observe", "read"}, {"Host", "measure", "probe"}} {
		scope := pkg.Scope
		for _, name := range path {
			sym, ok := scope.LookupLocal(name)
			if !ok || sym.Scope == nil {
				t.Fatalf("%v: %s not found", path, name)
			}
			scope = sym.Scope
		}
		if got := r.ThisContext(scope); got != host {
			t.Errorf("ThisContext(%v) = %v, want Host", path, got)
		}
	}
	standalone, _ := pkg.Scope.LookupLocal("standalone")
	step, _ := standalone.Scope.LookupLocal("step")
	if got := r.ThisContext(step.Scope); got != nil {
		t.Errorf("ThisContext(standalone::step) = %v, want nil: no object owns it", got)
	}
}
