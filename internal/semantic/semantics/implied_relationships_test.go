package semantics

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// impliedRelationshipRows returns one row (kind, owner, target) per implied relationship under root; an
// element is keyed as the pilot's exporter keys it: its qualified name, or owner/Metaclass[i] by position.
func impliedRelationshipRows(t *testing.T, m *Model, root *symbols.Scope) []string {
	t.Helper()
	keys := map[*symbols.Symbol]string{}
	key := func(sym *symbols.Symbol) string {
		if k, ok := keys[sym]; ok {
			return k
		}
		// A library element is keyed by its segments' effective names, quoted where no basic name.
		var segments []string
		for at := sym; at != nil; at = m.ownerOf(at) {
			name, named := m.EffectiveNamed(at)
			if !named {
				return m.resolver.Index().GetFQN(sym)
			}
			if !source.IsIdentifier(name) {
				name = source.UnrestrictedNameText(name)
			}
			segments = append([]string{name}, segments...)
		}
		return strings.Join(segments, "::")
	}
	var rows []string
	seen := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol)
	walk = func(sym *symbols.Symbol) {
		if seen[sym] {
			return
		}
		seen[sym] = true
		owned, _ := m.ReflectiveElements(sym, "ownedElement")
		// A range owns its bounds through memberships ownedElement does not read yet.
		owned = append(slices.Clone(owned), m.MultiplicityBounds(sym)...)
		// The elements held for implied relationships: binding connectors, their ends, the chains they own.
		// A transition's succession owns its ends through memberships ownedElement does not read yet.
		if transitionSuccessionPart(sym) {
			owned = append(owned, m.SuccessionEndMembers(sym)...)
		}
		owned = append(owned, impliedOwnedElements(m, sym)...)
		counts := map[string]int{}
		for _, child := range owned {
			meta := "?"
			if metaclass := m.MetaclassOf(child); metaclass != nil {
				meta = metaclass.Name
			}
			name, named := m.EffectiveNamed(child)
			if named && !strings.Contains(key(sym), "/") {
				keys[child] = key(sym) + "::" + name
			} else {
				keys[child] = fmt.Sprintf("%s/%s[%d]", key(sym), meta, counts[meta])
			}
			counts[meta]++
		}
		if sym.Kind != symbols.SymbolPackage {
			owner := key(sym)
			rows = append(rows, heldElementRows(m, sym, owner, key)...)
			for _, general := range m.ImpliedSpecializations(sym) {
				rows = append(rows, "Specialization\t"+owner+"\t"+key(general))
			}
			for _, redefined := range m.ImpliedRedefinitions(sym) {
				rows = append(rows, "Redefinition\t"+owner+"\t"+key(redefined))
				if !slices.Contains(m.DirectRedefinedFeatures(sym), redefined) {
					t.Errorf("%s: masking does not read the implied redefinition of %s", owner, key(redefined))
				}
			}
			derived := m.DerivedFeaturingTypes(sym)
			for _, featuring := range m.ImpliedTypeFeaturings(sym) {
				target := key(featuring.Type)
				if featuring.Snapshots {
					target = "snapshots(" + target + ")"
				}
				rows = append(rows, "TypeFeaturing\t"+owner+"\t"+target)
				if !slices.Contains(derived, featuring) {
					t.Errorf("%s: featuringType does not read the implied featuring by %s", owner, target)
				}
			}
		}
		for _, child := range owned {
			walk(child)
		}
		// A connector relates features keyed under its ends, so its row follows them.
		if _, related, ok := m.ImpliedBinding(sym); ok {
			rows = append(rows, "BindingConnector\t"+key(m.ownerOf(sym))+"\t"+key(related[0])+" = "+key(related[1]))
		}
	}
	root.WalkDeclarations(func(sym *symbols.Symbol) {
		if sym.OwnerScope == root {
			walk(sym)
		}
	})
	slices.Sort(rows)
	return slices.Compact(rows)
}

// pilotImpliedRows reads a fixture of the pilot's including export, one implied relationship
// per row, with the metaclasses of Specialization read as one kind.
func pilotImpliedRows(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		kind, rest, ok := strings.Cut(line, "\t")
		if !ok || strings.Count(rest, "\t") != 1 {
			t.Fatalf("%s: malformed row %q", path, line)
		}
		if !slices.Contains([]string{"Redefinition", "TypeFeaturing", "BindingConnector", "FeatureChaining"}, kind) {
			kind = "Specialization"
		}
		rows = append(rows, kind+"\t"+rest)
	}
	slices.Sort(rows)
	return slices.Compact(rows)
}

// impliedDifferenceReasons explains each reason testdata/implied_relationships/differences.txt
// gives for a row only the pilot states or only the model derives.
var impliedDifferenceReasons = map[string]string{
	"default-multiplicity": "the pilot states a usage's default multiplicity as a Multiplicity subsetting Base::exactlyOne; " +
		"checkMultiplicitySpecialization names Base::naturals, which is what the model derives",
	"suboccurrence": "checkFeatureSuboccurrenceSpecialization holds of a composite feature typed by a structure in a class; " +
		"the pilot implies nothing for it",
	"value-featuring": "checkExpressionTypeFeaturing gives a value expression the featuring types of its featureWithValue; " +
		"the pilot states none for the value of a variable feature",
	"lazy-operand": "the pilot owns the then and else operands of a conditional through a FeatureReferenceExpression referring to " +
		"the written expression; the model holds the written expression as the operand, so the rows under it differ",
	"sequence-nesting": "KerML 8.2.5.8.1: a sequence expression of more than two items nests, each ',' taking the rest as its second argument; " +
		"the model holds one ',' expression over all the items, so the rows under the nested ones differ",
	"loop-variable-subsetting": "the pilot has the variable of a for loop subset the loop's own seq parameter; no constraint states it: " +
		"checkForLoopActionUsageVarRedefinition names Actions::ForLoopAction::var alone, which the library has subset ForLoopAction::seq",
	"loop-variable-data-value": "checkFeatureDataValueSpecialization holds of a feature typed by a data type, the typed variable of a for loop included; " +
		"the pilot states none of a loop variable",
	"branch-action": "SysML.xtext ActionBodyParameter: an action written as the branch of a conditional is its thenClause or elseClause parameter; " +
		"the model holds the branch as an action of its own owning the written one, as its default export writes it",
	"binding-featuring": "the pilot states a featuring type for the binding connector of a feature reference, a result, a behavior invocation or a transition; " +
		"checkConnectorTypeFeaturing implies only the defaultFeaturingType, which is null where no featuring type has every related feature featured within it",
	"endless-connector": "the pilot has a binding connector or a succession written without ends specialize Links::links alone; " +
		"checkBindingConnectorSpecialization and checkSuccessionSpecialization name Links::selfLinks and Occurrences::happensBeforeLinks whatever the ends",
	"behavior-result-typing": "the pilot types the result of a behavior invocation by the behavior, and so has it subset Occurrences::occurrences; " +
		"checkInvocationExpressionBehaviorResultSpecialization asks that the result specialize the behavior, which types nothing",
	"satisfy-subject": "the pilot has the subject of a satisfied requirement redefine Requirements::RequirementCheck::subj beside the subject of the " +
		"requirement satisfied; the latter redefines the former already, and the model derives it alone",
	"trigger-invocation": "the model holds no parameter of a TriggerInvocationExpression and no function for it yet, so the rows of " +
		"checkTriggerInvocationExpressionSpecialization and of its parameters are the pilot's alone, and its featuring is its owning payload's",
	"typed-feature-base": "the test of a conditional or a loop redefines the library's ifTest, whileTest or untilTest, a feature typed by " +
		"BooleanEvaluation the model has subset Performances::booleanEvaluations and the pilot Base::things; " +
		"so the model finds Performances::evaluations, and the Evaluation::result its result would redefine, redundant (KerML 8.4.2)",
}

// impliedRelationshipDifferences reads differences.txt: side (pilot or model), fixture, the row,
// reason. It returns the reason of each fixture's rows keyed by side and row.
func impliedRelationshipDifferences(t *testing.T) map[string]map[string]string {
	t.Helper()
	path := filepath.Join("testdata", "implied_relationships", "differences.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 6 || (fields[0] != "pilot" && fields[0] != "model") {
			t.Fatalf("%s: malformed row %q", path, line)
		}
		if _, ok := impliedDifferenceReasons[fields[5]]; !ok {
			t.Fatalf("%s: %q has no entry in impliedDifferenceReasons", path, fields[5])
		}
		if out[fields[1]] == nil {
			out[fields[1]] = map[string]string{}
		}
		out[fields[1]][fields[0]+"\t"+strings.Join(fields[2:5], "\t")] = fields[5]
	}
	return out
}

// Every element of the fixtures owns the implied relationships the pilot's including export
// states for it (KerML Tables 8, 9 and 11; SysML Table 33), but for the listed differences.
func TestImpliedRelationshipsAgreeWithThePilotPerRow(t *testing.T) {
	differences := impliedRelationshipDifferences(t)
	reasons := map[string]bool{}
	for name, kind := range map[string]source.Kind{"rows.kerml": source.KindKerML, "rows.sysml": source.KindSysML, "expressions.sysml": source.KindSysML} {
		dir := filepath.Join("testdata", "implied_relationships")
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		pilot := pilotImpliedRows(t, filepath.Join(dir, name+".pilot.txt"))
		m, root := buildModelWithLibraryNamedKind(t, name, kind, string(src))
		got := impliedRelationshipRows(t, m, root)
		pinned := differences[name]
		for _, row := range got {
			if reason, differs := pinned["model\t"+row]; differs {
				reasons[reason] = true
				delete(pinned, "model\t"+row)
				if slices.Contains(pilot, row) {
					t.Errorf("%s: %q agrees with the pilot now; drop it from differences.txt", name, row)
				}
			} else if !slices.Contains(pilot, row) {
				t.Errorf("%s: %q is not in the pilot's export", name, row)
			}
		}
		for _, row := range pilot {
			if reason, differs := pinned["pilot\t"+row]; differs {
				reasons[reason] = true
				delete(pinned, "pilot\t"+row)
				if slices.Contains(got, row) {
					t.Errorf("%s: the pilot's %q is derived now; drop it from differences.txt", name, row)
				}
			} else if !slices.Contains(got, row) {
				t.Errorf("%s: the pilot's %q is not derived", name, row)
			}
		}
		for row := range pinned {
			t.Errorf("%s: differences.txt names %q, which neither side yields", name, row)
		}
	}
	for reason := range impliedDifferenceReasons {
		if !reasons[reason] {
			t.Errorf("impliedDifferenceReasons explains %q, which differences.txt does not use", reason)
		}
	}
}

// KerML §8.4.2 prunes implied Specializations only: an end redefines the end at its position of every
// general its owner states, and only a redefinition the notation writes leaves nothing to imply.
func TestImpliedRedefinitionsAreNotPruned(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "redefinitions.kerml", source.KindKerML, `package P {
		class C;
		assoc A { end a : C; end b : C; }
		assoc B specializes A { end c : C; end d : C; }
		assoc Both specializes A, B { end e : C; end f : C; }
		assoc Written specializes A { end g : C redefines a; end h : C; }
		assoc N { end x : C; end y : C; end [1] feature z : C; }
	}`)
	idx := m.resolver.Index()
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"P::A::a", []string{"Links::BinaryLink::source"}},
		{"P::B::c", []string{"P::A::a"}},
		{"P::Both::e", []string{"P::A::a", "P::B::c"}},
		{"P::Both::f", []string{"P::A::b", "P::B::d"}},
		{"P::Written::g", nil},
		{"P::N::x", nil},
		{"P::Written::h", []string{"P::A::b"}},
	} {
		sym := auditLibraryBases(t, root, tc.path)
		var got []string
		for _, redefined := range m.ImpliedRedefinitions(sym) {
			got = append(got, idx.GetFQN(redefined))
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: implied redefinitions %v, want %v", tc.path, got, tc.want)
		}
	}
	// The cross feature of an end among three is featured by the product of the other ends'
	// types, which is no type of the model: nothing is implied rather than one of them.
	if cross := m.OwnedCrossFeature(auditLibraryBases(t, root, "P::N::z")); cross == nil {
		t.Error("P::N::z owns no cross feature")
	} else {
		if got := m.ImpliedTypeFeaturings(cross); len(got) != 0 {
			t.Errorf("the cross feature of P::N::z: implied featurings %v, want none", got)
		}
	}
}

// KerML Table 9 classifies a feature by each type it has: one base per kind of type.
func TestImpliedSpecializationsOfAReferenceTypedTwice(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "typed.sysml", source.KindSysML, `package P {
		attribute def AD; occurrence def OD;
		part def Owner { ref r : AD, OD; }
	}`)
	idx := m.resolver.Index()
	var got []string
	for _, general := range m.ImpliedSpecializations(auditLibraryBases(t, root, "P::Owner::r")) {
		got = append(got, idx.GetFQN(general))
	}
	slices.Sort(got)
	if want := []string{"Base::dataValues", "Occurrences::occurrences"}; !slices.Equal(got, want) {
		t.Errorf("P::Owner::r: implied specializations %v, want %v", got, want)
	}
}

// KerML checkPayloadFeatureRedefinition holds for the payload a recorded flow declares as for one read from source.
func TestImpliedRedefinitionsOfARecordedFlowPayload(t *testing.T) {
	src := `package P { part def T; part a { out o : T; } part b { in i : T; } flow f of p : T from a.o to b.i; }`
	for i, fixture := range reflectiveFixtures(t, "payload.sysml", source.KindSysML, src) {
		payload := auditLibraryBases(t, fixture.root, "P::f::p")
		var got []string
		for _, redefined := range fixture.model.ImpliedRedefinitions(payload) {
			got = append(got, fixture.model.resolver.Index().GetFQN(redefined))
		}
		if want := []string{transferPayloadFQN}; !slices.Equal(got, want) {
			t.Errorf("fixture %d: implied redefinitions %v, want %v", i, got, want)
		}
	}
}

// impliedOwnedElements returns the elements sym owns only once implied relationships are included:
// its binding connectors, a connector's ends, and the chain an end or sym's implied relationship owns.
func impliedOwnedElements(m *Model, sym *symbols.Symbol) []*symbols.Symbol {
	out := m.ImpliedBindingConnectors(sym)
	if ends, _, ok := m.ImpliedBinding(sym); ok {
		out = append(out, ends[:]...)
	}
	return append(out, m.ImpliedOwnedFeatures(sym)...)
}

// heldElementRows returns the rows of what sym is as a held element: an
// end's reference and a chain's chaining features in order.
func heldElementRows(m *Model, sym *symbols.Symbol, owner string, key func(*symbols.Symbol) string) []string {
	var rows []string
	if referred := m.ImpliedEndReference(sym); referred != nil {
		rows = append(rows, "Specialization\t"+owner+"\t"+key(referred))
	}
	for i, link := range heldChainingFeatures(m, sym) {
		rows = append(rows, fmt.Sprintf("FeatureChaining\t%s\t[%d] %s", owner, i, key(link)))
	}
	return rows
}

// heldChainingFeatures returns the features a chain held for an implied relationship chains.
func heldChainingFeatures(m *Model, sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Inclusion == nil {
		return nil
	}
	return m.ChainingFeatures(sym)
}
