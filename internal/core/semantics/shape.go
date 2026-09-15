package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Kernel-library members frame every object (`Occurrence::self`, `portions`) and stay
// out of its shape; Systems, Domain and OpenSysML members describe it and are kept.

// fqnAnythingSelf is the feature every thing has of itself ([KerML] Base::Anything::self).
const fqnAnythingSelf = "Base::Anything::self"

// frameRoots names the Kernel features whose restatements stay in the frame: the
// object's identity, its history (time slices, snapshots, start and end) and the
// transfers it takes part in, which the runtime tracks itself.
var frameRoots = map[string]bool{
	fqnAnythingSelf:                              true,
	"Occurrences::Occurrence::timeSlices":        true,
	"Occurrences::Occurrence::incomingTransfers": true,
	"Occurrences::Occurrence::outgoingTransfers": true,
}

// ShapeFeature is one feature of an object's shape: the name it is held under
// and the declaration whose value it holds, which for a redefined name is the
// redefinition's target (a redefinition shares its target's value).
type ShapeFeature struct {
	Name string
	// Symbol is the last declaration of Name in the type's members, own then inherited.
	Symbol *symbols.Symbol
	// Declared is the first: the model's own restatement where it has one.
	Declared *symbols.Symbol
}

// ShapeFeatures returns the features an object of typ carries, in shape order:
// its own then its inherited members, less what is no feature, what frames every
// object, and the variants a variation offers.
func (m *Model) ShapeFeatures(typ *symbols.Symbol) []ShapeFeature {
	if m == nil || typ == nil {
		return nil
	}
	defer m.own(typ)()
	if cached, ok := m.shapes[typ]; ok {
		return cached
	}
	out := m.shapeFeatures(typ, func(member *symbols.Symbol) bool {
		return !m.FrameFeature(member) || m.DescribesReference(typ, member)
	})
	if m.MemberSourcesStable(typ) && m.computingRedefinedFeatures == 0 {
		journal(m, m.shapes, typ, typ.Decl)
		m.shapes[typ] = out
	}
	return out
}

// shapeFeatures collects typ's shape features in shape order, keeping the
// members keep admits.
func (m *Model) shapeFeatures(typ *symbols.Symbol, keep func(*symbols.Symbol) bool) []ShapeFeature {
	if typ == nil {
		return nil
	}
	var order []string
	byName := make(map[string]*ShapeFeature)
	seen := make(map[*symbols.Symbol]bool)
	for _, member := range m.MembersOfIncludingRedefined(typ) {
		if seen[member] {
			continue
		}
		seen[member] = true
		if !IsShapeFeature(member) || !keep(member) || m.VariationPointOwning(member) != nil {
			continue
		}
		if f, ok := byName[member.Name]; ok {
			f.Symbol = member
			continue
		}
		byName[member.Name] = &ShapeFeature{Name: member.Name, Symbol: member, Declared: member}
		order = append(order, member.Name)
	}
	out := make([]ShapeFeature, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}

// ConstructibleFeatures returns the features a constructor's positional
// arguments bind, in shape order: those declared or restated in the constructed
// type's own tier (the model's for a model type, a library's for one of its
// types), not the ones a more general library declares for every object of the kind.
// A redefinition and its target are one feature, so they take one position.
func (m *Model) ConstructibleFeatures(typ *symbols.Symbol) []*symbols.Symbol {
	return m.constructorSlots(typ).features
}

// ConstructibleFeatureFor returns the constructible feature of typ standing for
// feature (itself, or one it redefines or is redefined by); nil when none does.
func (m *Model) ConstructibleFeatureFor(typ, feature *symbols.Symbol) *symbols.Symbol {
	return m.constructorSlots(typ).slotOf[feature]
}

// constructorSlots is a type's constructible features and, for every declaration
// of one of them under any name, the feature listed for it.
type constructorSlots struct {
	features []*symbols.Symbol
	slotOf   map[*symbols.Symbol]*symbols.Symbol
}

// constructorSlots computes typ's constructor slots. Memoized.
func (m *Model) constructorSlots(typ *symbols.Symbol) constructorSlots {
	if m == nil || typ == nil || m.resolver == nil || m.resolver.Index() == nil {
		return constructorSlots{}
	}
	defer m.own(typ)()
	if cached, ok := m.ctorSlots[typ]; ok {
		return cached
	}
	slots := m.computeConstructorSlots(typ)
	if m.computingRedefinedFeatures == 0 {
		journal(m, m.ctorSlots, typ, typ.Decl)
		m.ctorSlots[typ] = slots
	}
	return slots
}

func (m *Model) computeConstructorSlots(typ *symbols.Symbol) constructorSlots {
	tier := m.libraryTier(typ)
	var declared []*symbols.Symbol
	for _, f := range m.shapeFeatures(typ, func(member *symbols.Symbol) bool { return m.libraryTier(member) == tier }) {
		if _, isUsage := f.Declared.Decl.(*ast.Usage); isUsage {
			declared = append(declared, f.Declared)
		}
	}
	// Union-find over declarations: the earliest positioned one leads its group;
	// one holding no position (masked, or of another tier) never does.
	position := make(map[*symbols.Symbol]int, len(declared))
	for i, decl := range declared {
		position[decl] = i
	}
	leader := make(map[*symbols.Symbol]*symbols.Symbol, len(declared))
	var find func(*symbols.Symbol) *symbols.Symbol
	find = func(decl *symbols.Symbol) *symbols.Symbol {
		next, ok := leader[decl]
		if !ok || next == decl {
			leader[decl] = decl
			return decl
		}
		root := find(next)
		leader[decl] = root
		return root
	}
	for _, decl := range declared {
		for _, redefined := range m.redefinedTransitively(decl) {
			if _, known := position[redefined]; !known {
				position[redefined] = len(position)
			}
			a, b := find(decl), find(redefined)
			switch {
			case a == b:
			case position[a] < position[b]:
				leader[b] = a
			default:
				leader[a] = b
			}
		}
	}
	features := make([]*symbols.Symbol, 0, len(declared))
	for _, decl := range declared {
		if find(decl) == decl {
			features = append(features, decl)
		}
	}
	slotOf := make(map[*symbols.Symbol]*symbols.Symbol, len(position))
	for decl := range position {
		slotOf[decl] = find(decl)
	}
	return constructorSlots{features: features, slotOf: slotOf}
}

// redefinedTransitively returns every feature sym redefines, by clause or by
// position (an end, a subject or objective), and those they redefine in turn.
func (m *Model) redefinedTransitively(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		for _, redefined := range [][]*symbols.Symbol{
			m.RedefinedFeatures(cur), m.ImplicitEndRedefinitions(cur), m.ImplicitRoleRedefinitions(cur),
		} {
			for _, target := range redefined {
				if target == nil || seen[target] {
					continue
				}
				seen[target] = true
				out = append(out, target)
				queue = append(queue, target)
			}
		}
	}
	return out
}

// IsShapeFeature reports whether sym is a kind of feature an object carries a
// value of (an attribute, part, port, action…), as opposed to a nested type,
// a calculation or an enumeration.
func IsShapeFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolPartUsage, symbols.SymbolItemUsage,
		symbols.SymbolPortUsage, symbols.SymbolConnectionUsage, symbols.SymbolActionUsage,
		symbols.SymbolStateUsage, symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage,
		symbols.SymbolOccurrenceUsage, symbols.SymbolIndividualUsage,
		symbols.SymbolInterfaceUsage, symbols.SymbolFlowUsage,
		// An allocation usage is a connection usage of the allocation library
		// (SysML v2 §8.3.19), so an object carries it as a feature.
		symbols.SymbolAllocationUsage:
		return true
	default:
		return false
	}
}

// IsValueType reports whether sym is a type whose instances are values rather
// than objects: an attribute or enumeration definition or usage.
func IsValueType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolAttributeDef, symbols.SymbolEnumerationDef,
		symbols.SymbolAttributeUsage, symbols.SymbolEnumerationUsage:
		return true
	default:
		return false
	}
}

// describedReferenceRoots are the value types whose library members describe
// the value an object of them is: a frame's axes and transformation, a scale's
// unit and mapping, a transformation's placement. A unit stays a scalar value.
var describedReferenceRoots = []string{
	"MeasurementReferences::VectorMeasurementReference",
	"MeasurementReferences::CoordinateTransformation",
	"MeasurementReferences::TranslationOrRotation",
	"MeasurementReferences::QuantityValueMapping",
	"MeasurementReferences::DefinitionalQuantityValue",
}

// referenceSelfRoots are the described-reference members that name the featuring
// frame itself (`transformation { :>> target = that; }`), which the runtime
// supplies as that frame rather than reads from the object.
var referenceSelfRoots = map[string]bool{
	"MeasurementReferences::CoordinateFrame::transformation::target": true,
}

// IsDescribedReference reports whether objects of typ carry their library
// members: a coordinate frame, a measurement scale, a coordinate transformation
// or a definitional value, but not a measurement unit.
func (m *Model) IsDescribedReference(typ *symbols.Symbol) bool {
	if m == nil || typ == nil {
		return false
	}
	if unit := m.libSymbol(fqnMeasurementUnit); unit != nil && m.Conforms(typ, unit) {
		return false
	}
	for _, fqn := range describedReferenceRoots {
		if root := m.libSymbol(fqn); root != nil && m.Conforms(typ, root) {
			return true
		}
	}
	return false
}

// DescribesReference reports whether member, a library attribute of a described
// reference type, is kept in typ's shape although a value type declares it.
func (m *Model) DescribesReference(typ, member *symbols.Symbol) bool {
	if member == nil || member.Kind != symbols.SymbolAttributeUsage || IsParameter(member) {
		return false
	}
	// The Kernel Semantic Library (Anything::that) frames a reference as it does
	// every object; the Collections and domain members describe it.
	if tier := m.libraryTier(member); tier == symbols.TierKernelSemantic || tier == symbols.TierLibrary {
		return false
	}
	if m.restatesRoot(member, frameRoots) || m.NamesFeaturingReference(member) {
		return false
	}
	// A usage nested in a described reference (a transformation's rotationMatrix)
	// describes it too.
	for cur := typ; cur != nil; cur = ownerOf(cur) {
		if m.IsDescribedReference(cur) {
			return true
		}
		if cur.Kind != symbols.SymbolAttributeUsage {
			return false
		}
	}
	return false
}

// FrameFeature reports whether a library-declared member frames the object
// rather than describing it, so that the shape leaves it out.
func (m *Model) FrameFeature(sym *symbols.Symbol) bool {
	tier := m.libraryTier(sym)
	switch {
	case !tier.Library():
		return false
	case tier.Frame():
		return true
	case m.HeldByValue(sym):
		return true
	case IsParameter(sym):
		return true
	default:
		return m.restatesRoot(sym, frameRoots) || m.NamesFeaturingReference(sym)
	}
}

// NamesFeaturingReference reports a member restating one of referenceSelfRoots:
// the frame featuring a transformation, which its value answers rather than its object.
func (m *Model) NamesFeaturingReference(sym *symbols.Symbol) bool {
	return m != nil && m.restatesRoot(sym, referenceSelfRoots)
}

// RestatesFeaturingReference reports whether typ's member for feature is one
// NamesFeaturingReference accepts: `target` read through a frame's transformation.
func (m *Model) RestatesFeaturingReference(typ, feature *symbols.Symbol) bool {
	if m == nil || typ == nil || feature == nil || !IsShapeFeature(feature) {
		return false
	}
	for _, member := range m.MembersOfIncludingRedefined(typ) {
		if IsShapeFeature(member) && m.restates(member, feature) && m.NamesFeaturingReference(member) {
			return true
		}
	}
	return false
}

// HeldByValue reports a member the value of a value-held library type carries itself (`num`,
// `mRefs`, a constraint); a stated (`isBound default false`) or record-typed one is its object's.
func (m *Model) HeldByValue(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil || !m.ValueHeld(sym.OwnerScope.Owner()) {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || (sym.Kind != symbols.SymbolAttributeUsage && sym.Kind != symbols.SymbolEnumerationUsage) {
		return true
	}
	return usage.Value == nil && m.ValueHeld(sym)
}

// ValueHeld reports whether sym is held as a value, not an object: a scalar, an enumeration, or
// a TensorQuantityValue/TensorMeasurementReference by specialization (frames and scales included).
func (m *Model) ValueHeld(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolEnumerationDef, symbols.SymbolEnumerationUsage:
		return true
	}
	if m.PrimTypeOf(sym) != PrimUnknown {
		return true
	}
	for _, fqn := range []string{fqnTensorQuantityValue, fqnTensorMeasurementReference} {
		if root := m.libSymbol(fqn); root != nil && m.Conforms(sym, root) {
			return true
		}
	}
	return false
}

// libraryTier reports the tier of the library that declares sym, TierNone for a
// declaration of the model under evaluation.
func (m *Model) libraryTier(sym *symbols.Symbol) symbols.LibraryTier {
	if m == nil || m.resolver == nil || m.resolver.Index() == nil {
		return symbols.TierNone
	}
	return m.resolver.Index().LibraryTier(sym)
}

// IsParameter reports whether sym is a directed or result parameter of a behavior.
func IsParameter(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && (usage.Direction != ast.DirNone || usage.IsResult)
}

// IsSelf reports whether sym is a thing's `self` feature: Base::Anything::self or a
// feature restating it, such as DataValue::self or a definition's own redefinition.
func (m *Model) IsSelf(sym *symbols.Symbol) bool {
	return m != nil && sym != nil && m.restatesAny(sym, namesFQN(fqnAnythingSelf))
}

// namesFQN accepts the symbol whose qualified name is fqn.
func namesFQN(fqn string) func(*symbols.Symbol) bool {
	return func(sym *symbols.Symbol) bool { return symbols.FQNOf(sym) == fqn }
}

// restatesRoot reports whether sym redefines or subsets one of the roots, directly
// or through the features those name.
func (m *Model) restatesRoot(sym *symbols.Symbol, roots map[string]bool) bool {
	return m.restatesAny(sym, func(sym *symbols.Symbol) bool { return roots[symbols.FQNOf(sym)] })
}

// restatesAny reports whether sym, or a feature it redefines or subsets transitively,
// is one the predicate accepts.
func (m *Model) restatesAny(sym *symbols.Symbol, root func(*symbols.Symbol) bool) bool {
	if sym == nil {
		return false
	}
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		if root(cur) {
			return true
		}
		for _, rel := range RelationshipsOf(cur) {
			if rel == nil || (rel.Kind != ast.RelRedefines && rel.Kind != ast.RelSubsets) {
				continue
			}
			if named := m.relationshipTarget(cur, rel); named != nil && !seen[named] {
				seen[named] = true
				queue = append(queue, named)
			}
		}
	}
	return false
}
