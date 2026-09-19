package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// resolveWithModel is buildModel returning the resolver, whose diagnostics say
// which references the masked view of inheritance left unresolved.
func resolveWithModel(t *testing.T, name, src string) *resolve.Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocumentWithKind(name, root, source.KindOf(name))
	r := resolve.New(idx)
	r.SetModel(NewModel(r))
	r.ResolveDocument(name, root)
	return r
}

// unresolvedNames returns the reference texts the resolver could not resolve.
func unresolvedNames(r *resolve.Resolver) []string {
	var out []string
	for _, d := range r.Diagnostics {
		if i := strings.Index(d.Message, "unresolved reference: "); i >= 0 {
			name := d.Message[i+len("unresolved reference: "):]
			if j := strings.Index(name, " "); j >= 0 {
				name = name[:j]
			}
			out = append(out, name)
		}
	}
	return out
}

// KerML 8.3.3.3: what a feature of a general redefines is not inherited further
// down the chain, so naming it from a subtype does not resolve.
func TestMaskedInheritedFeatureIsNotResolvableInSubtype(t *testing.T) {
	r := resolveWithModel(t, "t.kerml", `package p {
		classifier A { feature x; feature y; }
		classifier B specializes A { feature b redefines x; }
		classifier C specializes B {
			feature c1 redefines x;
			feature c2 redefines y;
		}
	}`)
	got := unresolvedNames(r)
	if len(got) != 1 || got[0] != "x" {
		t.Fatalf("unresolved = %v, want [x]: `x` is masked by B::b, `y` is not", got)
	}
}

// A declaration redefining an inherited namesake still names what that namesake
// redefines when another general still inherits it (KerML 7.3.4.5): the port
// redefines one typed by the general port definition, so `pwrCmd` reaches it.
func TestRedefiningNamesakeStillNamesItsTarget(t *testing.T) {
	r := resolveWithModel(t, "t.sysml", `package p {
		port def PwrCmdPort { in item pwrCmd; }
		port def FuelCmdPort :> PwrCmdPort { in item fuelCmd redefines pwrCmd; }
		part def V0 { port pwrCmdPort : PwrCmdPort; }
		part def V :> V0 {
			port fuelCmdPort : FuelCmdPort redefines pwrCmdPort {
				in item fuelCmd redefines pwrCmd;
			}
		}
	}`)
	if got := unresolvedNames(r); len(got) != 0 {
		t.Fatalf("unresolved = %v, want none", got)
	}
}

// Without that other general, an inherited redefinition masks the feature from
// every reference in the subtype, its own redefinitions included; the pinned
// pilot rejects all three references here.
func TestInheritedRedefinitionMasksEveryReference(t *testing.T) {
	r := resolveWithModel(t, "t.sysml", `package p {
		port def PwrCmdPort { in item pwrCmd; }
		port def FuelCmdPort :> PwrCmdPort { in item fuelCmd redefines pwrCmd; }
		part def V {
			port fuelCmdPort : FuelCmdPort { in item fuelCmd redefines pwrCmd; }
			port p2 : FuelCmdPort { in item other redefines pwrCmd; }
			port p3 : FuelCmdPort { in item other :> pwrCmd; }
		}
	}`)
	got := unresolvedNames(r)
	if len(got) != 3 {
		t.Fatalf("unresolved = %v, want pwrCmd three times", got)
	}
	for _, name := range got {
		if name != "pwrCmd" {
			t.Fatalf("unresolved = %v, want pwrCmd three times", got)
		}
	}
}
