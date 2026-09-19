package resolve

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestResolveQualifiedRequirement covers SysML v2 §8.2.2.19.2: an objective may
// require a requirement named by a qualified reference, and the body of that
// requirement may redefine a qualified feature of it.
func TestResolveQualifiedRequirement(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def MaxFuelMassRequirement {
			attribute actualFuelMass;
		}
		package FuelMassAnalysis {
			requirement fuelMassRequirement : MaxFuelMassRequirement;
		}
		analysis def FuelMassAnalysisCase {
			attribute calculatedFuelMass;
			objective fuelMassAnalysisObjective {
				require FuelMassAnalysis::fuelMassRequirement {
					:>> MaxFuelMassRequirement::actualFuelMass = calculatedFuelMass;
				}
				assume FuelMassAnalysis::fuelMassRequirement;
			}
		}
	}`)
	onlyDuplicateNameWarnings(t, r, 2)
}

// onlyDuplicateNameWarnings checks that r reports exactly n duplicate-name
// warnings and nothing else: both members of an objective that require and assume
// one requirement are named by it (matched run against the pilot).
func onlyDuplicateNameWarnings(t *testing.T, r *Resolver, n int) {
	t.Helper()
	for _, d := range r.Diagnostics {
		if !d.Warning || d.Code != CodeNameConflict {
			t.Fatalf("expected only duplicate-name warnings, got %v", r.Diagnostics)
		}
	}
	if len(r.Diagnostics) != n {
		t.Fatalf("expected %d duplicate-name warnings, got %v", n, r.Diagnostics)
	}
}

// The member reference-subsets the requirement it names, so its body redefines
// that requirement's features by their plain names too, not only by their
// qualified ones (SysML.xtext RequirementConstraintUsage).
func TestResolveRequiredRequirementFeatureByPlainName(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def MaxFuelMassRequirement {
			attribute actualFuelMass;
		}
		package FuelMassAnalysis {
			requirement fuelMassRequirement : MaxFuelMassRequirement;
		}
		analysis def FuelMassAnalysisCase {
			attribute calculatedFuelMass;
			objective fuelMassAnalysisObjective {
				require FuelMassAnalysis::fuelMassRequirement {
					:>> actualFuelMass = calculatedFuelMass;
				}
				assume FuelMassAnalysis::fuelMassRequirement {
					:>> actualFuelMass = calculatedFuelMass;
				}
			}
		}
	}`)
	onlyDuplicateNameWarnings(t, r, 2)
}

// A plain name the referenced requirement does not declare is still reported:
// its features are searched, not assumed.
func TestResolveRequiredRequirementUnknownPlainName(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		analysis def C {
			attribute m;
			objective o {
				require A::r {
					:>> missingFeature = m;
				}
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic for missingFeature")
	}
}

// Only the direct members of a require body inherit from the requirement it
// references, so a nested declaration never redefines that requirement's
// feature — its own type holds it.
func TestResolveNestedRedefinitionPrefersOwnType(t *testing.T) {
	src := `package Starkit {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		part def Payload { attribute mass; }
		analysis def C {
			objective o {
				require A::r {
					part p : Payload {
						:>> mass;
					}
				}
			}
		}
	}`
	p := parser.New(source.New("d.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("d.sysml", root)
	r := New(idx)
	r.ResolveDocument("d.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}

	qn := nestedRedefinitionTarget(t, root)
	sym, ok := r.PartSymbol(qn, 0)
	if !ok {
		t.Fatalf("the nested redefinition target was not resolved")
	}
	owner, _ := sym.OwnerScope.Node().(*ast.Definition)
	if owner == nil || owner.Ident.Name != "Payload" {
		t.Errorf("mass resolved in %v, want Payload", sym.OwnerScope.Node())
	}
}

// nestedRedefinitionTarget returns the redefinition target of the only
// redefinition declared inside a usage body in root.
func nestedRedefinitionTarget(t *testing.T, root *ast.RootNamespace) *ast.QualifiedName {
	t.Helper()
	var found *ast.QualifiedName
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, m := range members {
			if mem, ok := m.(*ast.Membership); ok {
				m = mem.Member
			}
			switch n := m.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Definition:
				walk(n.Members)
			case *ast.RequireMember:
				walk(n.Body)
			case *ast.Usage:
				for _, rel := range n.Relationships {
					if rel.Kind != ast.RelRedefines {
						continue
					}
					if qn, ok := rel.Target.(*ast.QualifiedName); ok && len(qn.Parts) == 1 {
						found = qn
					}
				}
				walk(n.Members)
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatalf("no plain-name redefinition found")
	}
	return found
}

// A required requirement that names no such member is reported, so a qualified
// target is resolved rather than accepted on sight.
func TestResolveQualifiedRequirementUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		package FuelMassAnalysis;
		analysis def C {
			objective o {
				require FuelMassAnalysis::missingRequirement;
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic for FuelMassAnalysis::missingRequirement")
	}
}

// A qualified redefinition target in a requirement body is resolved through the
// namespace it names, not by its last segment alone.
func TestResolveQualifiedRedefinitionUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		analysis def C {
			attribute m;
			objective o {
				require A::r {
					:>> R::missingFeature = m;
				}
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic for R::missingFeature")
	}
}

// TestResolveDragonStructures covers the structural notation Dragon.sysml
// exercises alongside the constructs added here: an allocation, a viewpoint
// definition and usage, a constraint usage with multiplicity, an assert of that
// constraint, and occurrence portions. Each must resolve, not merely parse.
func TestResolveDragonStructures(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Dragon {
		part a;
		part b;
		allocation allocate a to b;
		constraint def C;
		constraint c [1] : C {}
		assert constraint c;
		viewpoint def V;
		viewpoint v : V;
		item def Vehicle;
		item vehicle : Vehicle {
			attribute mass;
			timeslice item cruise;
			snapshot item takeoff;
		}
		interface def CommunicationInterface {
			end;
			end;
		}
	}`)
	// `assert constraint c` names the constraint usage above it a second time,
	// which the reference reports on both, as a warning (matched run, w6c).
	for _, d := range r.Diagnostics {
		if !d.Warning || d.Code != CodeNameConflict {
			t.Fatalf("expected only duplicate-name warnings, got %v", r.Diagnostics)
		}
	}
	if len(r.Diagnostics) != 2 {
		t.Fatalf("expected the two duplicate-name warnings for c, got %v", r.Diagnostics)
	}
}

// A reference whose qualifying namespace is not loaded says so, and names the
// declaration that does exist, rather than reading as a plain typo.
func TestResolveMissingStandardViewNamespace(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package P {
		view def GeneralView;
		package Views { alias gv for P::GeneralView; }
		view Model : 'SysML Standard Diagrams'::gv;
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected a diagnostic for an unloaded namespace")
	}
	msg := r.Diagnostics[0].Message
	for _, want := range []string{"SysML Standard Diagrams", "no namespace", "P::Views::gv"} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic %q does not mention %q", msg, want)
		}
	}
}

// A binding end is a feature chain whose segments are qualified names, as the
// Open-MBEE DesertKite model writes them. Every segment must resolve to the
// declaration it names — resolution walks the chain, it does not stop at the
// last segment.
func TestResolveQualifiedBindingChain(t *testing.T) {
	src := `package 'Desert Kites' {
		part 'Kite Environment' {
			part 'Region Earth Surface';
		}
		part 'Kite System' {
			part 'Desert Kite' {
				part 'Kite Wall' { attribute 'Wall Height'; }
			}
		}
		package 'Kite Requirements' { attribute 'Minimum Wall Height'; }

		binding bind 'Kite Environment'::'Region Earth Surface'.'Kite System'::'Desert Kite'.'Kite Wall'.'Wall Height'
			= 'Kite Requirements'::'Minimum Wall Height';
	}`
	p := parser.New(source.New("dk.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("dk.sysml", root))
	r.ResolveDocument("dk.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}

	binding := findBindingUsage(t, root.Members)
	if len(binding.ConnectorEnds) != 2 {
		t.Fatalf("expected two binding ends, got %v", binding.ConnectorEnds)
	}

	// Source end: the whole chain, outermost segment last.
	wantSource := []string{
		"Kite Environment", "Region Earth Surface",
		"Kite System", "Desert Kite",
		"Kite Wall", "Wall Height",
	}
	if got := resolvedSegments(t, r, binding.ConnectorEnds[0].AttachedTarget()); !equalStrings(got, wantSource) {
		t.Errorf("source end resolved to %v, want %v", got, wantSource)
	}

	wantTarget := []string{"Kite Requirements", "Minimum Wall Height"}
	if got := resolvedSegments(t, r, binding.ConnectorEnds[1].AttachedTarget()); !equalStrings(got, wantTarget) {
		t.Errorf("target end resolved to %v, want %v", got, wantTarget)
	}
}

// bindingEnd returns the feature attached at binding end i.
func bindingEnd(t *testing.T, binding *ast.Usage, i int) ast.Node {
	t.Helper()
	if i >= len(binding.ConnectorEnds) || binding.ConnectorEnds[i] == nil {
		t.Fatalf("binding has %d ends, want end %d", len(binding.ConnectorEnds), i)
	}
	return binding.ConnectorEnds[i].AttachedTarget()
}

func TestResolveLongFeatureChain(t *testing.T) {
	const segments = 50
	src := longFeatureChainSource(segments, -1, "")
	p := parser.New(source.New("long-chain.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("long-chain.sysml", root))
	r.ResolveDocument("long-chain.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}

	binding := findBindingUsage(t, root.Members)
	got := resolvedSegments(t, r, bindingEnd(t, binding, 0))
	want := []string{"A"}
	for i := 0; i < segments-2; i++ {
		want = append(want, "p"+strconv.Itoa(i))
	}
	want = append(want, "leaf")
	if !equalStrings(got, want) {
		t.Fatalf("chain resolved to %v, want %v", got, want)
	}
}

func TestResolveLongFeatureChainLookupCountIsLinear(t *testing.T) {
	counts := make(map[int]int)
	for _, segments := range []int{25, 50} {
		src := longFeatureChainSource(segments, -1, "")
		p := parser.New(source.New("count-chain.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics for %d segments: %v", segments, p.Diagnostics)
		}
		r := New(symbols.NewIndexFromDoc("count-chain.sysml", root))
		model := &countingMemberLookup{}
		r.SetModel(model)
		r.ResolveDocument("count-chain.sysml", root)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("resolve diagnostics for %d segments: %v", segments, r.Diagnostics)
		}
		counts[segments] = model.calls
		t.Logf("segments=%d member lookups=%d", segments, model.calls)
	}
	if counts[50] >= 3*counts[25] {
		t.Fatalf("member lookups grew too quickly: 25=%d, 50=%d", counts[25], counts[50])
	}
}

func TestResolveLongFeatureChainMissReportsItsOwnSpanOnce(t *testing.T) {
	const (
		segments = 30
		badAt    = 15
	)
	const missing = "missingSegment"
	src := longFeatureChainSource(segments, badAt, missing)
	p := parser.New(source.New("bad-long-chain.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("bad-long-chain.sysml", root))
	r.ResolveDocument("bad-long-chain.sysml", root)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %d: %v", len(r.Diagnostics), r.Diagnostics)
	}
	if got, want := r.Diagnostics[0].Span.Offset, strings.LastIndex(src, missing); got != want {
		t.Fatalf("diagnostic starts at %d, want bad segment offset %d", got, want)
	}
}

func TestResolveChainResolvesNonReferenceOperands(t *testing.T) {
	src := `package P {
		attribute indexed = missing[1].b;
		attribute invoked = missing(1).b;
	}`
	p := parser.New(source.New("non-reference-chain.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("non-reference-chain.sysml", root)
	r := New(idx)
	r.ResolveDocument("non-reference-chain.sysml", root)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("expected two unresolved diagnostics, got %d: %v", len(r.Diagnostics), r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if !strings.Contains(d.Message, "missing") {
			t.Errorf("diagnostic %q does not name the unresolved operand", d.Message)
		}
	}

	pkg, ok := idx.DocumentRoot("non-reference-chain.sysml").LookupLocal("P")
	if !ok {
		t.Fatal("package P was not indexed")
	}
	indexed, ok := pkg.Scope.LookupLocal("indexed")
	if !ok {
		t.Fatal("indexed attribute was not indexed")
	}
	if _, ok := indexed.Decl.(*ast.Usage).Value.(*ast.FeatureChainExpr).Operand.(*ast.IndexExpr); !ok {
		t.Fatal("indexed chain operand was not parsed as IndexExpr")
	}
	invoked, ok := pkg.Scope.LookupLocal("invoked")
	if !ok {
		t.Fatal("invoked attribute was not indexed")
	}
	if _, ok := invoked.Decl.(*ast.Usage).Value.(*ast.FeatureChainExpr).Operand.(*ast.InvocationExpr); !ok {
		t.Fatal("invoked chain operand was not parsed as InvocationExpr")
	}
}

type countingMemberLookup struct {
	calls int
}

func (m *countingMemberLookup) LookupMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	m.calls++
	return nil, false
}

func (m *countingMemberLookup) LookupContributedMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	return nil, false
}

func longFeatureChainSource(segments, missingAt int, missing string) string {
	var b strings.Builder
	b.WriteString("package C { part A {")
	for i := 0; i < segments-2; i++ {
		b.WriteString(" part p")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" {")
	}
	b.WriteString(" part leaf;")
	for i := 0; i < segments-2; i++ {
		b.WriteString(" }")
	}
	b.WriteString(" } part b; binding bind A")
	for i := 0; i < segments-2; i++ {
		b.WriteString(".")
		if missing != "" && i == missingAt {
			b.WriteString(missing)
		} else {
			b.WriteString("p")
			b.WriteString(strconv.Itoa(i))
		}
	}
	b.WriteString(".leaf = b; }")
	return b.String()
}

// A misspelled chain segment is one mistake, so it is reported once: the
// outward reading is only a probe until it is the one adopted.
func TestResolveChainBadQualifiedSegmentReportedOnce(t *testing.T) {
	src := `package P {
		part a { part b; }
		part c;
		binding bind a.'No Such'::x = c;
	}`
	p := parser.New(source.New("chain.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("chain.sysml", root))
	r.ResolveDocument("chain.sysml", root)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %d: %v", len(r.Diagnostics), r.Diagnostics)
	}
	if !strings.Contains(r.Diagnostics[0].Message, "No Such") {
		t.Errorf("diagnostic %q does not name the bad segment", r.Diagnostics[0].Message)
	}
}

// A segment reported unresolved leaves no symbol behind for a reader to jump to.
func TestResolveChainUnresolvedSegmentRecordsNoSymbol(t *testing.T) {
	src := `package P {
		part a;
		part B { part right; }
		part c;
		binding bind a.B::wrong = c;
	}`
	p := parser.New(source.New("probe.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("probe.sysml", root))
	r.ResolveDocument("probe.sysml", root)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected the chain to be reported unresolved")
	}

	binding := findBindingUsage(t, root.Members)
	chain, ok := bindingEnd(t, binding, 0).(*ast.FeatureChainExpr)
	if !ok {
		t.Fatalf("expected a feature chain end, got %T", bindingEnd(t, binding, 0))
	}
	if sym, ok := r.PartSymbol(chain.Member, 0); ok {
		t.Errorf("discarded reading left segment B pointing at %s", sym.Name)
	}
}

// A qualified chain segment names a member of the previous element when it has
// one, even where an outer declaration spells the same name.
func TestResolveChainPrefersMemberOverOuterDeclaration(t *testing.T) {
	src := `package P {
		part a { part B { part c; } }
		part B { part c; }
		part t;
		binding bind a.B::c = t;
	}`
	p := parser.New(source.New("inner.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("inner.sysml", root))
	r.ResolveDocument("inner.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}

	binding := findBindingUsage(t, root.Members)
	chain, ok := bindingEnd(t, binding, 0).(*ast.FeatureChainExpr)
	if !ok {
		t.Fatalf("expected a feature chain end, got %T", bindingEnd(t, binding, 0))
	}
	inner := innerSymbol(t, r, chain.Operand, "B")
	got, ok := r.PartSymbol(chain.Member, 0)
	if !ok {
		t.Fatal("segment B did not resolve")
	}
	if got != inner {
		t.Errorf("segment B resolved to the declaration in %v, want a's own member", got.OwnerScope)
	}
}

// innerSymbol returns the member the chain's previous element declares itself.
func innerSymbol(t *testing.T, r *Resolver, operand ast.Node, name string) *symbols.Symbol {
	t.Helper()
	ref, ok := operand.(*ast.FeatureReference)
	if !ok {
		t.Fatalf("expected a feature reference operand, got %T", operand)
	}
	operandSym, ok := r.PartSymbol(ref.Name, 0)
	if !ok {
		t.Fatal("chain operand did not resolve")
	}
	if operandSym.Scope == nil {
		t.Fatalf("%s has no scope", operandSym.Name)
	}
	sym, ok := operandSym.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s declares no member %q", operandSym.Name, name)
	}
	return sym
}

// resolvedSegments returns the name of the symbol each segment of a binding end
// resolved to, in source order.
func resolvedSegments(t *testing.T, r *Resolver, end ast.Node) []string {
	t.Helper()
	switch v := end.(type) {
	case *ast.FeatureChainExpr:
		return append(resolvedSegments(t, r, v.Operand), partNames(t, r, v.Member)...)
	case *ast.FeatureReference:
		return partNames(t, r, v.Name)
	case *ast.QualifiedName:
		return partNames(t, r, v)
	}
	t.Fatalf("unexpected binding end node %T", end)
	return nil
}

func partNames(t *testing.T, r *Resolver, qn *ast.QualifiedName) []string {
	t.Helper()
	var out []string
	for i, part := range qn.Parts {
		sym, ok := r.PartSymbol(qn, i)
		if !ok {
			t.Errorf("segment %q did not resolve", part.Text)
			out = append(out, "<unresolved>")
			continue
		}
		out = append(out, sym.Name)
	}
	return out
}

func findBindingUsage(t *testing.T, members []ast.Node) *ast.Usage {
	t.Helper()
	var found *ast.Usage
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, n := range nodes {
			switch v := n.(type) {
			case *ast.Membership:
				walk([]ast.Node{v.Member})
			case *ast.Package:
				walk(v.Members)
			case *ast.Usage:
				if v.Kind == ast.UsageBinding {
					found = v
					return
				}
				walk(v.Members)
			}
		}
	}
	walk(members)
	if found == nil {
		t.Fatal("no binding usage in parsed document")
	}
	return found
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
