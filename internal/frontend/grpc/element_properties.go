package grpc

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// GetElementProperties reads the metamodel properties of one element of a
// parsed model by their specification names, evaluated over the symbols the
// model resolved to (metamodel.SymbolStructure), one element at a time. A
// standard-library element — one a value referenced, or a library qualified
// name — is read from the form of the library document declaring it.
func (s *Service) GetElementProperties(ctx context.Context, req *pb.GetElementPropertiesRequest) (*pb.GetElementPropertiesResponse, error) {
	if err := s.requireCapability(CapabilityElementProperties); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	if strings.TrimSpace(req.ElementId) == "" {
		return nil, statusError(connect.CodeInvalidArgument, "element_id is required")
	}
	for _, name := range req.Properties {
		if strings.TrimSpace(name) == "" {
			return nil, statusError(connect.CodeInvalidArgument, "a property name is empty")
		}
	}
	reader := cached.elementReader()
	sym, ok := reader.lookup(req.ElementId)
	if ok && !cached.Index.Library(sym) {
		if excluded, refused := reader.excluded[sym.DocName]; refused {
			return &pb.GetElementPropertiesResponse{Error: excluded.Error, ExcludedDocuments: reader.exclusions}, nil
		}
		resp := reader.read(sym, req.Properties)
		resp.ExcludedDocuments = reader.exclusions
		return resp, nil
	}
	id := req.ElementId
	if ok && sym.Name != "" {
		id = cached.Index.GetFQN(sym)
	}
	resp, err := libraryElementProperties(id, req.Properties)
	if resp != nil {
		resp.ExcludedDocuments = reader.exclusions
	}
	return resp, err
}

// libraryElementProperties answers a standard-library element from the API
// element form of its own library document.
func libraryElementProperties(id string, properties []string) (*pb.GetElementPropertiesResponse, error) {
	element, form, known, err := export.LibraryElementForm(id)
	if err != nil {
		return &pb.GetElementPropertiesResponse{Error: err.Error()}, nil
	}
	if !known {
		return &pb.GetElementPropertiesResponse{Error: fmt.Sprintf("element not found: %s", id)}, nil
	}
	resp := &pb.GetElementPropertiesResponse{Element: libraryElementRefOf(form, element.ID)}
	if len(properties) == 0 {
		for _, property := range element.Properties {
			resp.Properties = append(resp.Properties, libraryElementPropertyOf(form, element, property.Name))
		}
		return resp, nil
	}
	for _, name := range properties {
		resp.Properties = append(resp.Properties, libraryElementPropertyOf(form, element, name))
	}
	return resp, nil
}

// elementReader evaluates the metamodel over a model's symbols. The semantic
// model and the evaluator memoize without locking, so reads take mu.
type elementReader struct {
	model *CachedModel
	// exclusions are the documents the model is read without, in document
	// order, and excluded indexes them by document name.
	exclusions []*pb.ExcludedDocument
	excluded   map[string]*pb.ExcludedDocument
	mu         sync.Mutex
	semantics  *semantics.Model
	// catalog fixes the ids of the bundled libraries' elements and memberships,
	// built on first use.
	catalog   *identity.Catalog
	structure *metamodel.SymbolStructure
	evaluator *metamodel.Evaluator
	// ids spells every element by the id the model's API element form (Convert)
	// writes for it, and resolves such an id back; nil when the form refuses the
	// model (idsRefusal), when the reader falls back to the symbols' own ids.
	ids        *export.SymbolIDs
	idsRefusal error
	addresses  map[*symbols.Symbol]string
	bySymbol   map[string]*symbols.Symbol
}

// elementReader is the model's symbol-backed property reader, built on first use.
func (m *CachedModel) elementReader() *elementReader {
	m.elementReaderOnce.Do(func() {
		resolver := resolve.New(m.Index)
		// The structure reads references through the resolver's record of the
		// documents (referencedSymbol), as the conversion does before it writes.
		for _, doc := range m.Documents {
			if doc != nil && doc.Source != nil && doc.Root != nil {
				resolver.ResolveDocument(doc.Source.Name(), doc.Root)
			}
		}
		sem := semantics.NewModel(resolver)
		sem.SetSourceText(cachedSourceText(m))
		structure := metamodel.NewSymbolStructure(resolver, sem)
		exclusions := m.documentExclusions()
		excluded := make(map[string]*pb.ExcludedDocument, len(exclusions))
		for _, exclusion := range exclusions {
			excluded[exclusion.Document] = exclusion
		}
		reader := &elementReader{
			model:      m,
			exclusions: exclusions,
			excluded:   excluded,
			semantics:  sem,
			structure:  structure,
			evaluator:  metamodel.New(resolver, sem, metamodel.Options{Structure: structure}),
			addresses:  map[*symbols.Symbol]string{},
			bySymbol:   map[string]*symbols.Symbol{},
		}
		reader.ids, reader.idsRefusal = m.symbolIDs(resolver, sem, excluded)
		if reader.ids != nil {
			structure.SetIdentities(reader.ids.Of)
			// The ownership walk reads the semantic model, which memoizes
			// without locking: done here, before the evaluator shares it.
			reader.ids.Reachable()
		}
		m.elementReaderValue = reader
	})
	return m.elementReaderValue
}

// symbolIDs spells the model's symbols by the ids Convert writes for the elements
// of the documents read (the excluded ones left out, so a top-level name two
// documents declare is no collision here); the error is why the form refuses
// those documents together (an id outside the alphabet).
func (m *CachedModel) symbolIDs(resolver *resolve.Resolver, sem *semantics.Model, excluded map[string]*pb.ExcludedDocument) (*export.SymbolIDs, error) {
	documents := make([]export.ModelDocument, 0, len(m.Documents))
	for _, doc := range m.Documents {
		if doc == nil || doc.Source == nil || doc.Root == nil || excluded[doc.Source.Name()] != nil {
			continue
		}
		documents = append(documents, export.ModelDocument{File: doc.Source, Root: doc.Root})
	}
	if len(documents) == 0 {
		return nil, nil
	}
	ids, err := export.NewElementIDs(documents)
	if err != nil {
		return nil, err
	}
	return export.NewSymbolIDs(ids, resolver, sem), nil
}

// documentExclusions is the documents the model is read without, in document order:
// one the parser could not read whole, one redeclaring an earlier document's top-level name.
func (m *CachedModel) documentExclusions() []*pb.ExcludedDocument {
	var out []*pb.ExcludedDocument
	excluded := map[string]bool{}
	exclude := func(name, why string) {
		if !excluded[name] {
			excluded[name] = true
			out = append(out, &pb.ExcludedDocument{Document: name, Error: why})
		}
	}
	order := map[string]int{}
	for i, doc := range m.Documents {
		if doc == nil || doc.Source == nil {
			continue
		}
		order[doc.Source.Name()] = i
		if syntax := convert.SyntaxErrorOf(doc.Source.Name(), doc.Source, doc.ParseDiags); syntax != nil {
			exclude(doc.Source.Name(), syntax.Error())
		}
	}
	declared := map[string]*symbols.Symbol{}
	var collisions []*symbols.Symbol
	for _, binding := range m.Index.TopLevelBindings("") {
		sym := binding.Sym
		if sym == nil || sym.DocName == "" || m.Index.Library(sym) {
			continue
		}
		if _, known := order[sym.DocName]; !known {
			continue
		}
		prior, taken := declared[binding.Name]
		switch {
		case !taken || prior.DocName == sym.DocName:
			declared[binding.Name] = sym
		case order[sym.DocName] < order[prior.DocName]:
			declared[binding.Name] = sym
			collisions = append(collisions, prior)
		default:
			collisions = append(collisions, sym)
		}
	}
	sort.SliceStable(collisions, func(i, j int) bool { return order[collisions[i].DocName] < order[collisions[j].DocName] })
	for _, sym := range collisions {
		prior := declared[m.Index.GetFQN(sym)]
		exclude(sym.DocName, fmt.Sprintf(
			"cannot read the declaration of %s at %s: its id lands on the same IRI as %s at %s, which another document declares, and merging two elements into one subject would be a different model",
			sym.Name, m.declarationAt(sym), prior.Name, m.declarationAt(prior)))
	}
	return out
}

// declarationAt spells where sym is declared, as document:line:col.
func (m *CachedModel) declarationAt(sym *symbols.Symbol) string {
	for _, doc := range m.Documents {
		if doc != nil && doc.Source != nil && doc.Source.Name() == sym.DocName {
			pos := doc.Source.Lines().PosAt(sym.DeclSpan.Offset)
			return fmt.Sprintf("%s:%d:%d", sym.DocName, pos.Line, pos.Col)
		}
	}
	return sym.DocName
}

// lookup finds the symbol a request addresses: the id the API element form writes for it, a
// symbol id the model resolves (qualified name, inherited member, positional id), or a path
// address this reader gave an element no id names.
func (r *elementReader) lookup(id string) (*symbols.Symbol, bool) {
	if sym, ok := r.ids.Symbol(id); ok {
		return sym, true
	}
	if syms := r.model.LookupNamedMember(id); len(syms) > 0 {
		return syms[0], true
	}
	if syms := r.model.LookupSymbol(id); len(syms) > 0 {
		return syms[0], true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pathSymbol(id, 0)
}

// A path address steps from an addressed element: pathSegmentRelationship to its n-th owned
// relationship, pathSegmentElement to the k-th element a relationship owns.
const (
	pathSegmentRelationship = ".r"
	pathSegmentElement      = ".e"
	maxPathDepth            = 64
)

// pathSymbol resolves a path address under r.mu: the prefix is an address of
// its own, the last segment a step from it.
func (r *elementReader) pathSymbol(id string, depth int) (*symbols.Symbol, bool) {
	if sym, ok := r.bySymbol[id]; ok {
		return sym, true
	}
	if depth > maxPathDepth {
		return nil, false
	}
	dot := strings.LastIndex(id, ".")
	if dot <= 0 || dot+2 >= len(id) {
		return nil, false
	}
	base, step := id[:dot], id[dot:dot+2]
	ordinal, err := strconv.Atoi(id[dot+2:])
	if err != nil || ordinal < 0 {
		return nil, false
	}
	var owner *symbols.Symbol
	if syms := r.model.LookupSymbol(base); len(syms) > 0 {
		owner = syms[0]
	} else if sym, ok := r.pathSymbol(base, depth+1); ok {
		owner = sym
	} else {
		return nil, false
	}
	var steps []metamodel.Element
	switch step {
	case pathSegmentRelationship:
		steps, _ = r.structure.OwnedRelationships(r.structure.ElementFor(owner))
	case pathSegmentElement:
		steps, _ = r.structure.OwnedRelatedElements(r.structure.ElementFor(owner))
	default:
		return nil, false
	}
	if ordinal >= len(steps) {
		return nil, false
	}
	sym, ok := r.structure.SymbolOf(steps[ordinal])
	if !ok {
		return nil, false
	}
	r.addresses[sym] = id
	r.bySymbol[id] = sym
	return sym, true
}

// address is the id a client reads sym back by, under r.mu: the id GetSymbol resolves, a
// library element's id, else a path from the nearest addressed owner ("" when none reaches one).
func (r *elementReader) address(sym *symbols.Symbol, depth int) string {
	if sym == nil {
		return ""
	}
	if id, ok := r.addresses[sym]; ok {
		return id
	}
	id := ""
	if r.model.Index.Library(sym) {
		// The id the library document's own form reads the element back by.
		if id, _ = r.structure.ElementIdentity(r.structure.ElementFor(sym)); id == "" {
			id = r.libraryElementID(sym)
		}
	} else if id, _ = r.ids.Of(sym); id == "" {
		id = r.model.SymbolContext().Identity(sym)
	}
	if id == "" && depth <= maxPathDepth {
		id = r.pathAddress(sym, depth)
	}
	r.addresses[sym] = id
	if id != "" {
		r.bySymbol[id] = sym
	}
	return id
}

func (r *elementReader) pathAddress(sym *symbols.Symbol, depth int) string {
	element := r.structure.ElementFor(sym)
	class, _ := r.structure.Metaclass(element)
	if r.structure.Specializes(class, "Relationship") {
		owner, present, _ := r.structure.OwningRelatedElement(element)
		if !present {
			return ""
		}
		siblings, _ := r.structure.OwnedRelationships(owner)
		return r.stepAddress(owner, siblings, element, pathSegmentRelationship, depth)
	}
	owning, present, _ := r.structure.OwningRelationship(element)
	if !present {
		return ""
	}
	siblings, _ := r.structure.OwnedRelatedElements(owning)
	return r.stepAddress(owning, siblings, element, pathSegmentElement, depth)
}

func (r *elementReader) stepAddress(from metamodel.Element, siblings []metamodel.Element, element metamodel.Element, step string, depth int) string {
	fromSym, ok := r.structure.SymbolOf(from)
	if !ok {
		return ""
	}
	base := r.address(fromSym, depth+1)
	if base == "" {
		return ""
	}
	for i, sibling := range siblings {
		if sibling.Key() == element.Key() {
			return base + step + strconv.Itoa(i)
		}
	}
	return ""
}

// read answers the properties of sym: the named ones, or every property its
// metaclass declares.
func (r *elementReader) read(sym *symbols.Symbol, properties []string) *pb.GetElementPropertiesResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	element := r.structure.ElementFor(sym)
	class, known := r.structure.Metaclass(element)
	if !known {
		return &pb.GetElementPropertiesResponse{Error: fmt.Sprintf("element not found: %s", r.model.Index.GetFQN(sym))}
	}
	resp := &pb.GetElementPropertiesResponse{Element: r.ref(sym, element)}
	if len(properties) == 0 {
		for _, definition := range ontology.Properties() {
			if ontology.IsAncestorOrSelf(class, definition.DefiningClass) {
				resp.Properties = append(resp.Properties, r.property(element, class, definition.Name))
			}
		}
		return resp
	}
	for _, name := range properties {
		resp.Properties = append(resp.Properties, r.property(element, class, name))
	}
	return resp
}

// property answers one property of an element: its values when derived, none when the metaclass
// declares it without a value, else why there is no answer (undeclared, not derivable, omitted).
func (r *elementReader) property(element metamodel.Element, class, name string) *pb.ElementProperty {
	out := &pb.ElementProperty{Name: name}
	definition, declared := ontology.PropertyOf(class, name)
	if !declared {
		out.Unsupported = fmt.Sprintf("%s declares no property %s", class, name)
		return out
	}
	out.DefiningClass = definition.DefiningClass
	out.Many = definition.Many
	out.Derived = definition.Derived
	if value, ok := r.evaluator.PropertyReadOnly(element, definition.DefiningClass, name); ok {
		out.Values = r.values(value)
		return out
	}
	if value, ok := r.statedProperty(element, definition); ok {
		out.Values = r.values(value)
		return out
	}
	if reason, explained := r.evaluator.Explain(element, definition.DefiningClass, name); explained {
		out.Unsupported = fmt.Sprintf("%s is not derived for this element: %s", definition.QualifiedName(), reason)
		return out
	}
	if reason, omitted := omittedPropertyReasons()[definition.QualifiedName()]; omitted {
		out.Unsupported = fmt.Sprintf("%s is not derived by OpenSysML: %s", definition.QualifiedName(), reason)
	}
	return out
}

// statedProperty reads a property the evaluator has no rule for straight from the notation:
// the element's identity, a stated attribute, or a stated relationship end.
func (r *elementReader) statedProperty(element metamodel.Element, definition ontology.Property) (metamodel.Value, bool) {
	if definition.Derived {
		return metamodel.Value{}, false
	}
	if definition.Name == "elementId" {
		id, ok := r.structure.ElementIdentity(element)
		return metamodel.Value{Kind: metamodel.StringValue, String: id, Success: true}, ok
	}
	if definition.Kind == ontology.ObjectProperty {
		related, ok := r.structure.StatedRelatedElements(element, definition.Name)
		if !ok {
			return metamodel.Value{}, false
		}
		values := make([]metamodel.Value, 0, len(related))
		for _, el := range related {
			values = append(values, metamodel.Value{Kind: metamodel.ElementValue, Element: el, Success: true})
		}
		return metamodel.Value{Kind: metamodel.SequenceValue, Values: values, Success: true}, true
	}
	return r.structure.Attribute(element, definition.Name)
}

// values spells an evaluated value as property values: a collection member
// per value, one value for a single-valued property, none for null.
func (r *elementReader) values(value metamodel.Value) []*pb.ElementPropertyValue {
	switch value.Kind {
	case metamodel.NullValue:
		return nil
	case metamodel.SequenceValue:
		out := make([]*pb.ElementPropertyValue, 0, len(value.Values))
		for _, member := range value.Values {
			out = append(out, r.values(member)...)
		}
		return out
	case metamodel.ElementValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_Element{Element: r.refOf(value.Element)}}}
	case metamodel.MembershipValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_Element{Element: r.refOf(metamodel.MembershipElement(value.Membership))}}}
	case metamodel.StringValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_StringValue{StringValue: value.String}}}
	case metamodel.EnumValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_StringValue{StringValue: value.Enum}}}
	case metamodel.BooleanValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_BoolValue{BoolValue: value.Boolean}}}
	case metamodel.IntegerValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_IntValue{IntValue: value.Integer}}}
	case metamodel.RealValue:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_RealValue{RealValue: value.Real}}}
	}
	return nil
}

// refOf identifies an element the evaluator answered with: one of this
// structure's, or a symbol the evaluator named directly.
func (r *elementReader) refOf(element metamodel.Element) *pb.ElementRef {
	if ref, ok := r.libraryMembershipRef(element); ok {
		return ref
	}
	sym, ok := r.structure.SymbolOf(element)
	if !ok && element.Symbol != nil {
		sym, ok = element.Symbol, true
		if element.IsMembership && element.Membership.Symbol != nil {
			sym = element.Membership.Symbol
		}
		element = r.structure.ElementFor(sym)
	}
	if !ok {
		ref := &pb.ElementRef{}
		ref.Metaclass, _ = r.structure.Metaclass(element)
		return ref
	}
	return r.ref(sym, element)
}

// libraryMembershipRef identifies a membership owning a library element: no
// symbol stands for it, so it carries the id the library document's own form
// writes for it (fixed for a catalogued member, minted for an unnamed one), the
// one id scheme every surface reads the membership by.
func (r *elementReader) libraryMembershipRef(element metamodel.Element) (*pb.ElementRef, bool) {
	membership := element.Membership
	if !element.IsMembership || membership.Symbol != nil || membership.Member == nil || !r.model.Index.Library(membership.Member) {
		return nil, false
	}
	ref := &pb.ElementRef{Metaclass: membership.Metaclass()}
	if library, ok := r.libraryCatalog().ElementForSymbol(membership.Member); ok && library != nil {
		ref.Id = library.OwningMembershipID
	} else if member, ok := identity.AnonymousMemberOf(r.semantics, r.libraryCatalog(), membership.Member); ok {
		ref.Id = member.OwningMembershipID
		if member.MembershipMetaclass != "" {
			ref.Metaclass = member.MembershipMetaclass
		}
	}
	if ref.Id == "" {
		return nil, false
	}
	if ref.Metaclass == "" {
		ref.Metaclass, _ = r.structure.Metaclass(element)
	}
	return ref, true
}

// libraryElementID is the id the library document's own form writes for sym
// when no identity names it: minted under its catalogued owner for an unnamed
// member, else its qualified name.
func (r *elementReader) libraryElementID(sym *symbols.Symbol) string {
	if sym.Name == "" {
		if member, ok := identity.AnonymousMemberOf(r.semantics, r.libraryCatalog(), sym); ok {
			return member.ID
		}
	}
	return r.model.Index.GetFQN(sym)
}

func (r *elementReader) libraryCatalog() *identity.Catalog {
	if r.catalog == nil {
		r.catalog = identity.LibraryCatalogWithOpenSysML(r.model.Index)
	}
	return r.catalog
}

// ref identifies the element standing for sym: the id a client reads it back
// by, its qualified name when it has one, and its metaclass.
func (r *elementReader) ref(sym *symbols.Symbol, element metamodel.Element) *pb.ElementRef {
	ref := &pb.ElementRef{Id: r.address(sym, 0), QualifiedName: r.qualifiedName(sym)}
	ref.Metaclass, _ = r.structure.Metaclass(element)
	return ref
}

// qualifiedName is the name that identifies sym, "" for an unnamed element.
func (r *elementReader) qualifiedName(sym *symbols.Symbol) string {
	if r.model.Index.Library(sym) {
		return r.model.Index.GetFQN(sym)
	}
	return r.model.Index.QualifiedIdentity(sym)
}

// libraryElementRefOf identifies a standard-library element by the library
// form's knowledge of its id.
func libraryElementRefOf(form *export.ElementForm, id string) *pb.ElementRef {
	ref := &pb.ElementRef{Id: id}
	ref.QualifiedName, ref.Metaclass, _ = form.NameOf(id)
	return ref
}

// libraryElementPropertyOf answers one property of a library element from its form: its values,
// none when the metaclass declares it without a value, else the reason there is no answer.
func libraryElementPropertyOf(form *export.ElementForm, element export.APIJSONElement, name string) *pb.ElementProperty {
	out := &pb.ElementProperty{Name: name}
	definition, declared := ontology.PropertyOf(element.Metaclass, name)
	if declared {
		out.DefiningClass = definition.DefiningClass
		out.Many = definition.Many
		out.Derived = definition.Derived
	}
	for _, property := range element.Properties {
		if property.Name != name {
			continue
		}
		out.Values = libraryElementPropertyValues(form, property.Value)
		return out
	}
	if !declared {
		out.Unsupported = fmt.Sprintf("%s declares no property %s", element.Metaclass, name)
		return out
	}
	if reason, omitted := form.Omission(element.ID, name); omitted {
		out.Unsupported = fmt.Sprintf("%s is not derived for this element: %s", definition.QualifiedName(), reason)
		return out
	}
	if reason, omitted := omittedPropertyReasons()[definition.QualifiedName()]; omitted {
		out.Unsupported = fmt.Sprintf("%s is not derived by OpenSysML: %s", definition.QualifiedName(), reason)
	}
	return out
}

// libraryElementPropertyValues spells a form value as property values.
func libraryElementPropertyValues(form *export.ElementForm, value any) []*pb.ElementPropertyValue {
	switch v := value.(type) {
	case nil:
		return nil
	case []any:
		out := make([]*pb.ElementPropertyValue, 0, len(v))
		for _, member := range v {
			out = append(out, libraryElementPropertyValues(form, member)...)
		}
		return out
	case export.APIJSONReference:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_Element{Element: libraryElementRefOf(form, v.ID)}}}
	case string:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_StringValue{StringValue: v}}}
	case bool:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_BoolValue{BoolValue: v}}}
	case int64:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_IntValue{IntValue: v}}}
	case float64:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_RealValue{RealValue: v}}}
	}
	return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_Number{Number: fmt.Sprint(value)}}}
}

// omittedPropertyReasons keys the metamodel evaluator's omissions by the
// property's qualified name, "Connector::connectorEnd".
var omittedPropertyReasons = sync.OnceValue(func() map[string]string {
	reasons := map[string]string{}
	for _, omission := range metamodel.Omitted() {
		reasons[omission.DefiningClass+"::"+omission.Property] = omission.Reason
	}
	return reasons
})
