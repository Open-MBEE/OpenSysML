package metamodel

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// symbolAspectPrefix marks an element handle a SymbolStructure hands out; the
// suffix is the element's position in the structure's table.
const symbolAspectPrefix = "sym:"

// symbolStructureIDs numbers the structures so a handle names the one table it
// indexes: `sym:<structure>:<position>`.
var symbolStructureIDs atomic.Uint64

// SymbolStructure is a Structure projected lazily from the symbols of a resolved
// model: declared symbols and the relationship and expression symbols the
// semantic model reflects, with no graph in between.
type SymbolStructure struct {
	res    *resolve.Resolver
	model  *semantics.Model
	prefix string

	mu        sync.Mutex
	elements  []symbolElement
	positions map[symbolKey]int
	conforms  map[[2]string]bool
	features  map[[2]string]*metaclassFeature
}

// symbolElement is one row of the structure's table: the symbol, the handle
// that stands for it, and its metaclass.
type symbolElement struct {
	sym     *symbols.Symbol
	element Element
	class   string
	known   bool
	library bool
}

// symbolKey identifies a symbol by what it reflects, so a relationship or
// expression symbol the semantic model synthesizes twice keeps one position.
type symbolKey struct {
	implicit symbols.ImplicitRelationship
	expr     symbols.ExpressionElement
	node     ast.Node
	chain    *symbols.ChainingFeature
	sym      *symbols.Symbol
	kind     symbols.SymbolKind
	form     uint8
}

// metaclassFeature is what the library's metaclass declaration says about one
// feature of a metaclass, resolved once per (metaclass, feature).
type metaclassFeature struct {
	sym     *symbols.Symbol
	element bool
	derived bool
	many    bool
	lower   int
}

// NewSymbolStructure makes the symbol-backed structure of the model res and
// model were analyzed as.
func NewSymbolStructure(res *resolve.Resolver, model *semantics.Model) *SymbolStructure {
	return &SymbolStructure{
		res:       res,
		model:     model,
		prefix:    symbolAspectPrefix + strconv.FormatUint(symbolStructureIDs.Add(1), 10) + ":",
		positions: make(map[symbolKey]int),
		conforms:  make(map[[2]string]bool),
		features:  make(map[[2]string]*metaclassFeature),
	}
}

// isSubjectDeclaration reports a `subject` parameter, a ReferenceUsage under a
// SubjectMembership (SysML 8.3.17.11) whatever kind the symbol table files it as.
func isSubjectDeclaration(sym *symbols.Symbol) bool {
	if sym.Implicit != nil || sym.Expr != nil {
		return false
	}
	_, ok := sym.Decl.(*ast.SubjectMember)
	return ok
}

func keyOf(sym *symbols.Symbol) symbolKey {
	switch {
	case sym.Implicit != nil:
		return symbolKey{implicit: *sym.Implicit, form: 1}
	case sym.Expr != nil:
		return symbolKey{expr: *sym.Expr, form: 2}
	case sym.Decl != nil:
		return symbolKey{node: sym.Decl, chain: sym.Chain, kind: sym.Kind, form: 3}
	}
	return symbolKey{sym: sym, form: 4}
}

// ElementFor is the handle standing for sym, placed on first use.
func (s *SymbolStructure) ElementFor(sym *symbols.Symbol) Element {
	if s == nil || sym == nil {
		return Element{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.place(sym).element
}

// All reads of the semantic model happen under s.mu: the model memoizes
// without locking, so the structure serializes evaluator goroutines itself.
func (s *SymbolStructure) elementFor(sym *symbols.Symbol) Element {
	if sym == nil {
		return Element{}
	}
	return s.place(sym).element
}

// place files sym in the table under s.mu and returns its row.
func (s *SymbolStructure) place(sym *symbols.Symbol) symbolElement {
	key := keyOf(sym)
	if position, ok := s.positions[key]; ok {
		return s.elements[position]
	}
	position := len(s.elements)
	aspect := s.prefix + strconv.Itoa(position)
	row := symbolElement{sym: sym}
	switch _, marker := s.model.InitialNodeTarget(sym); {
	case marker:
		row.class, row.known = "Membership", true
	case isSubjectDeclaration(sym):
		row.class, row.known = "ReferenceUsage", true
	default:
		if meta := s.model.MetaclassOf(sym); meta != nil && meta.Name != "" {
			row.class, row.known = meta.Name, true
		}
	}
	row.library = s.res != nil && s.res.Index() != nil && s.res.Index().Library(sym)
	if row.known && s.specializes(row.class, "Membership") {
		row.element = MembershipElement(Membership{Aspect: aspect})
	} else {
		row.element = Element{Aspect: aspect}
	}
	s.positions[key] = position
	s.elements = append(s.elements, row)
	return row
}

// SymbolOf is the symbol an element handed out by this structure stands for.
func (s *SymbolStructure) symbolOfLocked(element Element) (*symbols.Symbol, bool) {
	row, ok := s.row(element)
	if !ok {
		return nil, false
	}
	return row.sym, true
}

func (s *SymbolStructure) row(element Element) (symbolElement, bool) {
	if s == nil {
		return symbolElement{}, false
	}
	aspect := element.Aspect
	if element.IsMembership {
		aspect = element.Membership.Aspect
	}
	if !strings.HasPrefix(aspect, s.prefix) {
		return symbolElement{}, false
	}
	position, err := strconv.Atoi(strings.TrimPrefix(aspect, s.prefix))
	if err != nil || position < 0 || position >= len(s.elements) || s.elements[position].element.Key() != element.Key() {
		return symbolElement{}, false
	}
	return s.elements[position], true
}

// elementsOf is the handle of each symbol; a `first a;` marker a relationship
// end names stands for the action a, which the marker is a Membership of.
func (s *SymbolStructure) elementsOf(syms []*symbols.Symbol) []Element {
	out := make([]Element, 0, len(syms))
	for _, sym := range syms {
		if sym == nil {
			continue
		}
		if target, marker := s.model.InitialNodeTarget(sym); marker {
			sym = target
		}
		out = append(out, s.elementFor(sym))
	}
	return out
}

// relationshipsOf is the handle of each owned relationship; a `first a;`
// marker is itself the Membership the owner holds, not a member of one.
func (s *SymbolStructure) relationshipsOf(rels []*symbols.Symbol) []Element {
	out := make([]Element, 0, len(rels))
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if rel.Implicit != nil && rel.Implicit.Form == symbols.ImplicitMembership && rel.Implicit.Member != nil {
			if _, marker := s.model.InitialNodeTarget(rel.Implicit.Member); marker {
				rel = rel.Implicit.Member
			}
		}
		out = append(out, s.elementFor(rel))
	}
	return out
}

// markerRelated reads a relationship end of a `first a;` marker as the
// Membership it is: its namespace as source, the action it names as member.
func (s *SymbolStructure) markerRelated(sym *symbols.Symbol, property string) ([]*symbols.Symbol, bool) {
	target, marker := s.model.InitialNodeTarget(sym)
	if !marker {
		return nil, false
	}
	owner := s.model.OwningRelationship(sym)
	if owner != nil && owner.Implicit != nil {
		owner = owner.Implicit.Owner
	}
	switch property {
	case "memberElement", "target":
		return []*symbols.Symbol{target}, true
	case "source", "membershipOwningNamespace", "owningRelatedElement", "owner", "owningNamespace":
		if owner == nil {
			return nil, true
		}
		return []*symbols.Symbol{owner}, true
	case "relatedElement":
		if owner == nil {
			return []*symbols.Symbol{target}, true
		}
		return []*symbols.Symbol{owner, target}, true
	case "ownedRelatedElement", "ownedRelationship", "ownedElement", "owningRelationship", "owningMembership":
		return nil, true
	}
	return nil, false
}

// related is ReflectiveElements with the marker projection applied first; an
// invocation's `operand` (KerML 1.0 naming) reads its `argument` (KerML 1.1).
func (s *SymbolStructure) related(sym *symbols.Symbol, property string) ([]*symbols.Symbol, bool) {
	if syms, ok := s.markerRelated(sym, property); ok {
		return syms, true
	}
	if syms, ok := s.model.ReflectiveElements(sym, property); ok {
		return syms, true
	}
	if property == "operand" {
		return s.model.ReflectiveElements(sym, "argument")
	}
	return nil, false
}

// Metaclass is the metaclass the semantic model classifies the element's symbol by.
func (s *SymbolStructure) metaclassLocked(element Element) (string, bool) {
	row, ok := s.row(element)
	if !ok {
		return "", false
	}
	return row.class, row.known
}

// Specializes reports whether the metaclass sub is super or conforms to it in
// the loaded metamodel libraries.
func (s *SymbolStructure) Specializes(sub, super string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.specializes(sub, super)
}

func (s *SymbolStructure) specializes(sub, super string) bool {
	if sub == super {
		return true
	}
	key := [2]string{sub, super}
	if cached, ok := s.conforms[key]; ok {
		return cached
	}
	result := false
	if a, b := s.model.Metaclass(sub), s.model.Metaclass(super); a != nil && b != nil {
		result = s.model.Conforms(a, b)
	}
	s.conforms[key] = result
	return result
}

// OwnedRelationships are the relationships the element's symbol owns, in
// notation order (KerML 8.3.2.1 Element::ownedRelationship).
func (s *SymbolStructure) ownedRelationshipsLocked(element Element) ([]Element, bool) {
	row, ok := s.row(element)
	if !ok || row.library {
		return nil, false
	}
	if _, marker := s.model.InitialNodeTarget(row.sym); marker {
		return []Element{}, true
	}
	return s.relationshipsOf(s.model.OwnedRelationships(row.sym)), true
}

// OwningRelationship is the relationship owning the element's symbol, if any.
func (s *SymbolStructure) owningRelationshipLocked(element Element) (Element, bool, bool) {
	row, ok := s.row(element)
	if !ok || row.library {
		return Element{}, false, false
	}
	rel := s.model.OwningRelationship(row.sym)
	if _, marker := s.model.InitialNodeTarget(row.sym); rel == nil || marker {
		return Element{}, false, true
	}
	return s.elementFor(rel), true, true
}

// OwnedRelatedElements are the elements a relationship owns (KerML 8.3.2.1
// Relationship::ownedRelatedElement).
func (s *SymbolStructure) ownedRelatedElementsLocked(element Element) ([]Element, bool) {
	row, ok := s.row(element)
	if !ok || row.library {
		return nil, false
	}
	if related, ok := s.related(row.sym, "ownedRelatedElement"); ok {
		return s.elementsOf(related), true
	}
	if !row.known {
		return nil, false
	}
	return []Element{}, true
}

// OwningRelatedElement is the element owning a relationship (KerML 8.3.2.1
// Relationship::owningRelatedElement).
func (s *SymbolStructure) owningRelatedElementLocked(element Element) (Element, bool, bool) {
	row, ok := s.row(element)
	if !ok || row.library {
		return Element{}, false, false
	}
	if owner, present, known := s.oneRelated(row.sym, "owningRelatedElement"); !known || present {
		return owner, present, known
	}
	property := ""
	switch {
	case s.specializes(row.class, "Membership"):
		property = "membershipOwningNamespace"
	case row.class == "FeatureChaining":
		property = "owner"
	case row.class == "Annotation":
		property = "annotatedElement"
	}
	if property != "" {
		if owner, present, known := s.oneRelated(row.sym, property); !known || present {
			return owner, present, known
		}
	}
	if row.sym.Implicit != nil && row.sym.Implicit.Owner != nil {
		return s.elementFor(row.sym.Implicit.Owner), true, true
	}
	return Element{}, false, true
}

// oneRelated reads a single-valued element feature: present when it holds one
// element, known when the semantic model derives the feature at all.
func (s *SymbolStructure) oneRelated(sym *symbols.Symbol, property string) (Element, bool, bool) {
	related, ok := s.related(sym, property)
	switch {
	case !ok:
		return Element{}, false, true
	case len(related) == 0:
		return Element{}, false, true
	case len(related) != 1:
		return Element{}, false, false
	}
	return s.elementFor(related[0]), true, true
}

// RelatedElements reads an element-valued feature of the element's metaclass as
// the semantic model derives it from the notation.
func (s *SymbolStructure) relatedElementsLocked(element Element, property string) ([]Element, bool) {
	row, ok := s.row(element)
	if !ok || row.library || !row.known || strings.HasPrefix(row.class, ExtensionMetaclassPrefix) {
		return nil, false
	}
	if related, ok := s.related(row.sym, property); ok {
		return s.elementsOf(related), true
	}
	if feature := s.feature(row.class, property); feature != nil && feature.element {
		return []Element{}, true
	}
	return nil, false
}

// StatedRelatedElements reads a non-derived element feature the notation states.
func (s *SymbolStructure) statedRelatedElementsLocked(element Element, property string) ([]Element, bool) {
	row, ok := s.row(element)
	if !ok || !row.known || strings.HasPrefix(row.class, ExtensionMetaclassPrefix) {
		return nil, false
	}
	feature := s.feature(row.class, property)
	if feature == nil || !feature.element || feature.derived {
		return nil, false
	}
	related, ok := s.related(row.sym, property)
	if !ok {
		return nil, false
	}
	return s.elementsOf(related), true
}

// HasStatedRelatedElement reports a non-derived element feature the notation states a value for.
func (s *SymbolStructure) hasStatedRelatedElementLocked(element Element, property string) bool {
	related, ok := s.statedRelatedElementsLocked(element, property)
	return ok && len(related) > 0
}

// HasUnresolvedRelatedElement reports a written relationship whose end the
// notation names but the model does not resolve.
func (s *SymbolStructure) hasUnresolvedRelatedElementLocked(element Element, property string) bool {
	row, ok := s.row(element)
	if !ok || row.sym.Implicit == nil || row.sym.Implicit.Form != symbols.ImplicitWritten {
		return false
	}
	feature := s.feature(row.class, property)
	if feature == nil || !feature.element {
		return false
	}
	_, ok = s.model.ReflectiveElements(row.sym, property)
	return !ok
}

// Attribute reads a data-valued feature of the element's metaclass from the
// notation, at the metamodel default where the notation states nothing.
func (s *SymbolStructure) attributeLocked(element Element, property string) (Value, bool) {
	row, ok := s.row(element)
	if !ok || !row.known || strings.HasPrefix(row.class, ExtensionMetaclassPrefix) {
		return Value{}, false
	}
	name := symbolAttributeName(property)
	feature := s.feature(row.class, name)
	if feature != nil && feature.element {
		return Value{}, false
	}
	if value, ok := s.model.ReflectiveFeatureValue(row.sym, name); ok && s.reflectsAttribute(row.sym, name) {
		if name == "direction" && value.Kind == symbols.FilterValueEmpty && semantics.DefaultsToInputDirection(row.sym) {
			return Value{Kind: StringValue, String: "in", Success: true}, true
		}
		return s.attributeValue(value, feature)
	}
	if feature == nil {
		return Value{}, false
	}
	return s.unstatedAttribute(feature)
}

// reflectsAttribute reports whether sym's reflected value of name is the
// element's own: a `first a;` marker reflects the action's name, not the Membership's.
func (s *SymbolStructure) reflectsAttribute(sym *symbols.Symbol, name string) bool {
	if _, marker := s.model.InitialNodeTarget(sym); !marker {
		return true
	}
	return name == "isLibraryElement" || name == "isImplied" || name == "isImpliedIncluded"
}

func (s *SymbolStructure) unstatedAttribute(feature *metaclassFeature) (Value, bool) {
	switch {
	case feature == nil:
		return Value{Kind: NullValue, Success: true}, true
	case feature.lower > 0:
		return Value{}, false
	case feature.many:
		return Value{Kind: SequenceValue, Values: []Value{}, Success: true}, true
	}
	return Value{Kind: NullValue, Success: true}, true
}

func (s *SymbolStructure) attributeValue(value symbols.FilterValue, feature *metaclassFeature) (Value, bool) {
	switch value.Kind {
	case symbols.FilterValueBool:
		return Value{Kind: BooleanValue, Boolean: value.Bool, Success: true}, true
	case symbols.FilterValueInt:
		return Value{Kind: IntegerValue, Integer: value.Int, Success: true}, true
	case symbols.FilterValueReal:
		return Value{Kind: RealValue, Real: value.Real, Success: true}, true
	case symbols.FilterValueString:
		return Value{Kind: StringValue, String: value.Str, Success: true}, true
	case symbols.FilterValueRef:
		name := value.RefFQN
		if i := strings.LastIndex(name, "::"); i >= 0 {
			name = name[i+2:]
		}
		return Value{Kind: StringValue, String: name, Success: true}, true
	case symbols.FilterValueEmpty:
		return s.unstatedAttribute(feature)
	}
	return Value{}, false
}

// StatedAttribute is Attribute for a value the notation itself states.
func (s *SymbolStructure) statedAttributeLocked(element Element, property string) (Value, bool) {
	row, ok := s.row(element)
	if !ok || !row.known {
		return Value{}, false
	}
	name := symbolAttributeName(property)
	if !s.reflectsAttribute(row.sym, name) {
		return Value{}, false
	}
	value, ok := s.model.ReflectiveFeatureValue(row.sym, name)
	if !ok || value.Kind == symbols.FilterValueEmpty || value.Kind == symbols.FilterValueUnknown {
		return Value{}, false
	}
	return s.attributeLocked(element, property)
}

func symbolAttributeName(property string) string {
	switch property {
	case "name":
		return "declaredName"
	case "shortName":
		return "declaredShortName"
	}
	return property
}

// DerivedName is the effective name of the element's symbol.
func (s *SymbolStructure) derivedNameLocked(element Element) (string, bool) {
	row, ok := s.row(element)
	if !ok {
		return "", false
	}
	if _, marker := s.model.InitialNodeTarget(row.sym); marker {
		return "", false
	}
	value, ok := s.model.ReflectiveFeatureValue(row.sym, "name")
	if !ok || value.Kind != symbols.FilterValueString || value.Str == "" {
		return "", false
	}
	return value.Str, true
}

// LibrarySymbol is the symbol of an element of a bundled library.
func (s *SymbolStructure) librarySymbolLocked(element Element) (*symbols.Symbol, bool) {
	row, ok := s.row(element)
	if !ok || !row.library {
		return nil, false
	}
	return row.sym, true
}

// QualifiedName is the qualified name of a library element the model refers to.
func (s *SymbolStructure) qualifiedNameLocked(element Element) (string, bool) {
	row, ok := s.row(element)
	if !ok || !row.library || s.res == nil || s.res.Index() == nil {
		return "", false
	}
	fqn := s.res.Index().GetFQN(row.sym)
	return fqn, fqn != ""
}

// ElementIdentity is the stable id of a named declared element (identity package).
func (s *SymbolStructure) elementIdentityLocked(element Element) (string, bool) {
	row, ok := s.row(element)
	if !ok || row.sym.Implicit != nil || row.sym.Expr != nil || row.sym.Name == "" {
		return "", false
	}
	info, ok := identity.Of(s.model, s.res, row.sym)
	if !ok || info == nil || info.EffectiveID == "" {
		return "", false
	}
	return info.EffectiveID, true
}

// feature resolves one feature of a metaclass from the library declaration,
// once per (metaclass, feature); nil where the metaclass declares none.
func (s *SymbolStructure) feature(class, name string) *metaclassFeature {
	key := [2]string{class, name}
	if cached, ok := s.features[key]; ok {
		return cached
	}
	var resolved *metaclassFeature
	if meta := s.model.Metaclass(class); meta != nil {
		for _, member := range s.model.MembersOfIncludingRedefined(meta) {
			if member == nil || member.Name != name {
				continue
			}
			resolved = s.describeFeature(member)
			break
		}
	}
	s.features[key] = resolved
	return resolved
}

func (s *SymbolStructure) describeFeature(member *symbols.Symbol) *metaclassFeature {
	feature := &metaclassFeature{sym: member}
	if derived, ok := s.model.ReflectiveFeatureValue(member, "isDerived"); ok && derived.Kind == symbols.FilterValueBool {
		feature.derived = derived.Bool
	}
	types := s.model.FeatureTypes(member)
	feature.element = len(types) > 0
	for _, typ := range types {
		if s.model.IsDataType(typ) {
			feature.element = false
		}
	}
	if bounds, ok := s.model.MultiplicityOf(member); ok {
		if bounds.Lower.Known && !bounds.Lower.Infinite {
			feature.lower = int(bounds.Lower.Value)
		}
		feature.many = bounds.Upper.Infinite || (bounds.Upper.Known && bounds.Upper.Value > 1)
	}
	return feature
}

// Exported reads take s.mu once; the *Locked bodies and their helpers assume it is held.
func (s *SymbolStructure) Metaclass(element Element) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metaclassLocked(element)
}

func (s *SymbolStructure) OwnedRelationships(element Element) ([]Element, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ownedRelationshipsLocked(element)
}

func (s *SymbolStructure) OwningRelationship(element Element) (Element, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owningRelationshipLocked(element)
}

func (s *SymbolStructure) OwnedRelatedElements(element Element) ([]Element, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ownedRelatedElementsLocked(element)
}

func (s *SymbolStructure) OwningRelatedElement(element Element) (Element, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owningRelatedElementLocked(element)
}

func (s *SymbolStructure) RelatedElements(element Element, property string) ([]Element, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.relatedElementsLocked(element, property)
}

func (s *SymbolStructure) StatedRelatedElements(element Element, property string) ([]Element, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statedRelatedElementsLocked(element, property)
}

func (s *SymbolStructure) HasStatedRelatedElement(element Element, property string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasStatedRelatedElementLocked(element, property)
}

func (s *SymbolStructure) HasUnresolvedRelatedElement(element Element, property string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasUnresolvedRelatedElementLocked(element, property)
}

func (s *SymbolStructure) Attribute(element Element, property string) (Value, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attributeLocked(element, property)
}

func (s *SymbolStructure) StatedAttribute(element Element, property string) (Value, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statedAttributeLocked(element, property)
}

func (s *SymbolStructure) DerivedName(element Element) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.derivedNameLocked(element)
}

func (s *SymbolStructure) LibrarySymbol(element Element) (*symbols.Symbol, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.librarySymbolLocked(element)
}

func (s *SymbolStructure) QualifiedName(element Element) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.qualifiedNameLocked(element)
}

func (s *SymbolStructure) ElementIdentity(element Element) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.elementIdentityLocked(element)
}

func (s *SymbolStructure) SymbolOf(element Element) (*symbols.Symbol, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.symbolOfLocked(element)
}
