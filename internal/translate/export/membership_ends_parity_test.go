package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A membership's ends and member names are written as the reflective read derives them:
// source is the owning namespace, target the member, memberName the member's effective name.
func TestMembershipEndsReadAsTheConversionWritesThem(t *testing.T) {
	text := `package P {
	part def A;
	part <s> a : A;
	part def B { attribute x : A; }
	attribute v = 1;
	action def Shoot { in item image; }
	action shoot : Shoot { in item; }
}
`
	sf := source.New("members.sysml", []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	index := libs.NewModelIndex()
	index.AddDocument(sf.Name(), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.ResolveDocument(sf.Name(), root)
	graph, err := ToRDF(sf, root)
	if err != nil {
		t.Fatalf("to rdf: %v", err)
	}
	termOf := map[string]rdf.Term{}
	for _, subject := range graph.Subjects() {
		for _, qn := range graph.Objects(subject, rdf.SysML+"qualifiedName") {
			termOf[qn.Value] = subject
		}
	}
	membershipOf := func(member rdf.Term) rdf.Term {
		for _, subject := range graph.Subjects() {
			for _, class := range graph.Objects(subject, rdf.RDFType) {
				if strings.HasSuffix(class.Value, "Membership") || strings.HasSuffix(class.Value, "FeatureValue") {
					for _, m := range graph.Objects(subject, rdf.SysML+"ownedMemberElement") {
						if m == member {
							return subject
						}
					}
				}
			}
		}
		t.Fatalf("no membership owns %v", member)
		return rdf.Term{}
	}
	pkg, _ := index.DocumentRoot(sf.Name()).LookupLocal("P")
	b, _ := pkg.Scope.LookupLocal("B")
	v, _ := pkg.Scope.LookupLocal("v")
	type member struct {
		om, sym            *symbols.Symbol
		owner, member, via string
	}
	var cases []member
	for _, name := range []string{"A", "a", "B", "v"} {
		sym, _ := pkg.Scope.LookupLocal(name)
		cases = append(cases, member{model.OwningMembership(sym), sym, "P", "P::" + name, ""})
	}
	x, _ := b.Scope.LookupLocal("x")
	cases = append(cases, member{model.OwningMembership(x), x, "P::B", "P::B::x", ""})
	for _, rel := range firstElements(model.ReflectiveElements(v, "ownedRelationship")) {
		if model.MetaclassOf(rel).Name == "FeatureValue" {
			cases = append(cases, member{rel, firstElements(model.ReflectiveElements(rel, "target"))[0], "P::v", "", "value"})
		}
	}
	shoot, _ := pkg.Scope.LookupLocal("shoot")
	// An unnamed parameter is named by the one it redefines (KerML 8.3.3.3.4): memberName "image" on both ledgers.
	unnamed := shoot.Scope.AllMembers()[0]
	cases = append(cases, member{model.OwningMembership(unnamed), unnamed, "P::shoot", "", "ownedMember"})
	if values, _ := model.ReflectiveFeatureValues(cases[len(cases)-1].om, "memberName"); len(values) != 1 || values[0].Str != "image" {
		t.Errorf("memberName of shoot's unnamed parameter = %v, want image", values)
	}
	for _, c := range cases {
		sym, om := c.sym, c.om
		if om == nil {
			t.Fatalf("%s has no native owning membership", c.member)
		}
		src, _ := model.ReflectiveElements(om, "source")
		tgt, _ := model.ReflectiveElements(om, "target")
		rel, _ := model.ReflectiveElements(om, "relatedElement")
		if len(src) != 1 || len(tgt) != 1 || tgt[0] != sym || len(rel) != 2 || rel[0] != src[0] || rel[1] != sym {
			t.Fatalf("%s: native source %v target %v relatedElement %v", c.member, src, tgt, rel)
		}
		owner := termOf[c.owner]
		memberTerm := termOf[c.member]
		if c.member == "" {
			memberTerm = graph.Objects(owner, rdf.SysML+c.via)[0]
		}
		membership := membershipOf(memberTerm)
		if got := graph.Objects(membership, rdf.SysML+"source"); len(got) != 1 || got[0] != owner {
			t.Errorf("%s: written source %v, want %v", sym.Name, got, owner)
		}
		if got := graph.Objects(membership, rdf.SysML+"target"); len(got) != 1 || got[0] != memberTerm {
			t.Errorf("%s: written target %v, want %v", sym.Name, got, memberTerm)
		}
		if got := graph.Objects(membership, rdf.SysML+"relatedElement"); len(got) != 2 || got[0] != owner || got[1] != memberTerm {
			t.Errorf("%s: written relatedElement %v, want %v then %v", sym.Name, got, owner, memberTerm)
		}
		for _, property := range []string{"memberName", "memberShortName"} {
			var native string
			if values, _ := model.ReflectiveFeatureValues(om, property); len(values) == 1 {
				native = values[0].Str
			}
			var written string
			if got := graph.Objects(membership, rdf.SysML+property); len(got) == 1 {
				written = got[0].Value
			}
			if native != written {
				t.Errorf("%s: written %s %q, native %q", sym.Name, property, written, native)
			}
		}
	}
}

func firstElements(elements []*symbols.Symbol, _ bool) []*symbols.Symbol { return elements }
