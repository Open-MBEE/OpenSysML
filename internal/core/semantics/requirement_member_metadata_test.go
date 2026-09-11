package semantics_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// requirementMemberSrc annotates a subject, an assume and a require member with
// a semantic metadata keyword (SysML v2 §7.27.3) and with plain metadata.
const requirementMemberSrc = `package P {
	private import Metaobjects::SemanticMetadata;
	part def Vehicle;
	part vehicles : Vehicle[*];
	metadata def vehicle :> SemanticMetadata {
		:>> baseType = vehicles meta SysML::Usage;
	}
	constraint def C;
	constraint checks : C[*];
	metadata def check :> SemanticMetadata {
		:>> baseType = checks meta SysML::Usage;
	}
	metadata def Reviewed;
	requirement def R {
		subject #vehicle v : Vehicle;
		assume #check constraint a : C;
		require #check constraint r : C;
	}
	requirement def Plain {
		subject #Reviewed v : Vehicle;
		assume #Reviewed constraint a : C;
		require #Reviewed constraint r : C { @Reviewed; }
	}
}`

// requirementMemberModel indexes src against the bundled libraries, which is
// what makes Metaobjects::SemanticMetadata and SysML::Usage resolve.
func requirementMemberModel(t *testing.T, src string) (*semantics.Model, *symbols.Index) {
	t.Helper()
	const name = "<t>.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx
}

func lookupFQN(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

func supertypeNames(m *semantics.Model, sym *symbols.Symbol) []string {
	var names []string
	for _, sup := range m.DirectSupertypes(sym) {
		names = append(names, symbols.FQNOf(sup))
	}
	return names
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// A prefix on a subject, assume or require member is a metadata annotation of
// the member, exactly as a prefix on an ordinary usage is.
func TestMetadataAnnotationsOfRequirementMembers(t *testing.T) {
	_, idx := requirementMemberModel(t, requirementMemberSrc)
	cases := []struct {
		fqn  string
		decl string
		want []string
	}{
		{"P::R::v", "*ast.SubjectMember", []string{"vehicle"}},
		{"P::R::a", "*ast.AssumeMember", []string{"check"}},
		{"P::R::r", "*ast.RequireMember", []string{"check"}},
		{"P::Plain::r", "*ast.RequireMember", []string{"Reviewed", "Reviewed"}},
	}
	for _, tc := range cases {
		sym := lookupFQN(t, idx, tc.fqn)
		switch sym.Decl.(type) {
		case *ast.SubjectMember, *ast.AssumeMember, *ast.RequireMember:
		default:
			t.Errorf("%s declared by %T, want %s", tc.fqn, sym.Decl, tc.decl)
		}
		annots := semantics.MetadataAnnotationsOf(sym.Decl)
		if len(annots) != len(tc.want) {
			t.Fatalf("%s: %d annotations, want %d", tc.fqn, len(annots), len(tc.want))
		}
		for i, a := range annots {
			if got := a.Node.Type.Parts[len(a.Node.Type.Parts)-1].Text; got != tc.want[i] {
				t.Errorf("%s annotation %d names %q, want %q", tc.fqn, i, got, tc.want[i])
			}
			if wantPrefix := i == 0; a.Prefix != wantPrefix {
				t.Errorf("%s annotation %d prefix = %v, want %v", tc.fqn, i, a.Prefix, wantPrefix)
			}
		}
	}
}

// A semantic metadata keyword on a subject, assume or require member makes the
// member subset the keyword's baseType usage (SysML v2 §7.27.3, §7.27.4).
func TestSemanticMetadataOnRequirementMembersSubsetsBaseType(t *testing.T) {
	m, idx := requirementMemberModel(t, requirementMemberSrc)
	cases := []struct{ fqn, typ, base string }{
		{"P::R::v", "P::Vehicle", "P::vehicles"},
		{"P::R::a", "P::C", "P::checks"},
		{"P::R::r", "P::C", "P::checks"},
	}
	for _, tc := range cases {
		got := supertypeNames(m, lookupFQN(t, idx, tc.fqn))
		if !containsName(got, tc.typ) || !containsName(got, tc.base) {
			t.Errorf("supertypes of %s = %v, want both %s and %s", tc.fqn, got, tc.typ, tc.base)
		}
	}
}

// Plain metadata annotates without specializing anything, on these members as
// on any usage: the keyword's definition is no supertype.
func TestPlainMetadataOnRequirementMembersAddsNoBase(t *testing.T) {
	m, idx := requirementMemberModel(t, requirementMemberSrc)
	for _, fqn := range []string{"P::Plain::v", "P::Plain::a", "P::Plain::r"} {
		got := supertypeNames(m, lookupFQN(t, idx, fqn))
		for _, n := range got {
			if n == "P::Reviewed" || n == "P::vehicles" || n == "P::checks" {
				t.Errorf("supertypes of %s = %v, want no metadata-derived base", fqn, got)
			}
		}
	}
}

// An anonymous `assume`/`require constraint` carries its prefix too: the
// anonymous constraint usage is annotated, and a semantic keyword gives it the
// keyword's base, as on an anonymous `constraint { … }`.
func TestMetadataOnAnonymousRequirementConstraints(t *testing.T) {
	m, idx := requirementMemberModel(t, `package P {
		private import Metaobjects::SemanticMetadata;
		constraint def C;
		constraint checks : C[*];
		metadata def check :> SemanticMetadata {
			:>> baseType = checks meta SysML::Usage;
		}
		metadata def Reviewed;
		requirement def R {
			assume #check constraint { true }
			require #check #Reviewed constraint : C { true }
		}
	}`)
	r := lookupFQN(t, idx, "P::R")
	anon := r.Scope.AnonymousMembers()
	if len(anon) != 2 {
		t.Fatalf("anonymous members of R = %d, want 2", len(anon))
	}
	for i, want := range [][]string{{"check"}, {"check", "Reviewed"}} {
		annots := semantics.MetadataAnnotationsOf(anon[i].Decl)
		if len(annots) != len(want) {
			t.Fatalf("anonymous member %d: %d annotations, want %d", i, len(annots), len(want))
		}
		for j, a := range annots {
			if got := a.Node.Type.Parts[len(a.Node.Type.Parts)-1].Text; got != want[j] || !a.Prefix {
				t.Errorf("anonymous member %d annotation %d = %q (prefix %v), want prefix %q", i, j, got, a.Prefix, want[j])
			}
		}
		got := supertypeNames(m, anon[i])
		if !containsName(got, "P::checks") || containsName(got, "P::Reviewed") {
			t.Errorf("supertypes of anonymous member %d = %v, want P::checks and no P::Reviewed", i, got)
		}
	}
	if got := supertypeNames(m, anon[1]); !containsName(got, "P::C") {
		t.Errorf("supertypes of the typed anonymous require = %v, want P::C", got)
	}
}

// The constraint usage an assume or require member owns has a constraint's
// implicit base, and redefines what its `:>>` names.
func TestRequirementConstraintUsageBases(t *testing.T) {
	m, idx := requirementMemberModel(t, `package P {
		constraint plain;
		requirement def R {
			assume constraint a;
			require constraint r;
		}
		requirement def S :> R {
			assume constraint a2 :>> a;
			require constraint r2 :>> r;
		}
	}`)
	want := supertypeNames(m, lookupFQN(t, idx, "P::plain"))
	if !containsName(want, "Constraints::ConstraintCheck") {
		t.Fatalf("supertypes of an ordinary constraint usage = %v, want Constraints::ConstraintCheck", want)
	}
	for _, fqn := range []string{"P::R::a", "P::R::r"} {
		got := supertypeNames(m, lookupFQN(t, idx, fqn))
		if len(got) != len(want) || !containsName(got, "Constraints::ConstraintCheck") {
			t.Errorf("supertypes of %s = %v, want %v", fqn, got, want)
		}
	}
	for fqn, redefined := range map[string]string{"P::S::a2": "P::R::a", "P::S::r2": "P::R::r"} {
		got := supertypeNames(m, lookupFQN(t, idx, fqn))
		if !containsName(got, redefined) {
			t.Errorf("supertypes of %s = %v, want %s", fqn, got, redefined)
		}
	}
}

// A prefix written ahead of the keyword is a syntax error, but the member the
// parser recovers keeps the annotation and its semantic base, so the editor's
// resolution and validation carry on past the misplaced run.
func TestMisplacedMetadataOnRequirementMembersStillAnnotatesOnRecovery(t *testing.T) {
	src := strings.NewReplacer(
		"subject #vehicle v", "#vehicle subject v",
		"assume #check constraint a", "#check assume constraint a",
		"require #check constraint r", "#check require constraint r",
	).Replace(requirementMemberSrc)
	const name = "<t>.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 3 {
		t.Fatalf("parse diagnostics = %v, want one per misplaced run", p.Diagnostics)
	}
	for _, d := range p.Diagnostics {
		if !strings.HasPrefix(d.Message, "prefix metadata follows '") || src[d.Span.Offset] != '#' {
			t.Errorf("diagnostic %q at %q, want the placement error at the '#'", d.Message, src[d.Span.Offset:d.Span.End()])
		}
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)

	cases := []struct{ fqn, keyword, base string }{
		{"P::R::v", "vehicle", "P::vehicles"},
		{"P::R::a", "check", "P::checks"},
		{"P::R::r", "check", "P::checks"},
	}
	for _, tc := range cases {
		sym := lookupFQN(t, idx, tc.fqn)
		annots := semantics.MetadataAnnotationsOf(sym.Decl)
		if len(annots) != 1 || !annots[0].Prefix || annots[0].Node.Type.Parts[len(annots[0].Node.Type.Parts)-1].Text != tc.keyword {
			t.Errorf("%s: annotations = %+v, want the one prefix %q", tc.fqn, annots, tc.keyword)
		}
		if got := supertypeNames(m, sym); !containsName(got, tc.base) {
			t.Errorf("supertypes of %s = %v, want %s", tc.fqn, got, tc.base)
		}
	}
}

// A semantic metadata definition binding no baseType of its own inherits its
// supertype's, so a keyword specializing a keyword gives the same base; a rebinding wins.
func TestSemanticMetadataInheritsBaseType(t *testing.T) {
	m, idx := requirementMemberModel(t, `package P {
		private import Metaobjects::SemanticMetadata;
		part def Vehicle;
		part vehicles : Vehicle[*];
		part trucks : Vehicle[*] :> vehicles;
		metadata def vehicle :> SemanticMetadata {
			:>> baseType = vehicles meta SysML::Usage;
		}
		metadata def car :> vehicle;
		metadata def truck :> vehicle {
			:>> baseType = trucks meta SysML::Usage;
		}
		#car part c : Vehicle;
		#truck part t : Vehicle;
	}`)
	if got := supertypeNames(m, lookupFQN(t, idx, "P::c")); !containsName(got, "P::vehicles") {
		t.Errorf("supertypes of P::c = %v, want the inherited base P::vehicles", got)
	}
	got := supertypeNames(m, lookupFQN(t, idx, "P::t"))
	if !containsName(got, "P::trucks") || containsName(got, "P::vehicles") {
		t.Errorf("supertypes of P::t = %v, want the rebound base P::trucks only", got)
	}
}
