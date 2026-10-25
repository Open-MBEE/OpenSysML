package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// resolveStdlibDoc resolves src over the standard library and returns the
// model, the document root and the member chain names left unresolved.
func resolveStdlibDoc(t *testing.T, name, src string) (*Model, *symbols.Scope, []string) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := stdlibIndex(t)
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	var unresolved []string
	for _, d := range r.Diagnostics {
		if i := strings.Index(d.Message, "unresolved member: "); i >= 0 {
			unresolved = append(unresolved, d.Message[i+len("unresolved member: "):])
		}
	}
	return m, idx.DocumentRoot(name), unresolved
}

// A case role redefines the library's role parameter (SysML v2 §7.22), and a
// redefined feature is not inherited under its name, by the redefinition's owner
// or by what specializes it (KerML §7.3.4.5, §8.3.3.1.10): `u.subj` is unresolved
// beside `u.s`, as `b.x` is beside `attribute y :>> x`, while an unnamed
// redefinition takes the redefined name, so the chain `u.obj` reads `objective { }`.
func TestRedefinedRoleNameIsNotInherited(t *testing.T) {
	m, root, unresolved := resolveStdlibDoc(t, "roles.sysml", `package P {
		part def A { attribute x : ScalarValues::Integer; }
		part def B :> A { attribute y :>> x; }
		part b : B;
		use case def U { subject s : ScalarValues::Integer; objective { doc /* o */ } }
		use case u : U;
		use case def Named { subject s : ScalarValues::Integer; objective o { doc /* o */ } }
		use case named : Named;
		requirement def RD { subject x; }
		requirement r : RD;
		ref q1 = b.x;
		ref q2 = b.y;
		ref q3 = u.subj;
		ref q4 = u.s;
		ref q5 = u.obj;
		ref q6 = named.subj;
		ref q7 = named.obj;
		ref q8 = named.o;
		ref q9 = r.subj;
		ref q10 = r.x;
	}`)
	want := []string{"x", "subj", "subj", "obj", "subj"}
	if strings.Join(unresolved, ",") != strings.Join(want, ",") {
		t.Fatalf("unresolved members = %v, want %v", unresolved, want)
	}
	u := sym(t, sym(t, root, "P").Scope, "u")
	if s, ok := m.LookupMember(u, "subj"); ok {
		t.Fatalf("u.subj resolved to %s; the subject s redefines it", s.Name)
	}
	s, ok := m.LookupMember(u, "s")
	if !ok || s.Name != "s" {
		t.Fatalf("u.s = %v, %v; want the declared subject", s, ok)
	}
	if names := visibleNames(m, u); names["subj"] != 0 || names["s"] != 1 {
		t.Fatalf("u offers %v; want s once and no subj", names)
	}
}
