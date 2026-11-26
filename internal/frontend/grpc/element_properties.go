package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// GetElementProperties reads one element's metamodel properties by their specification names over the symbols the model resolved
// to (metamodel.SymbolStructure), a library element from its document's form; an excluded document's element answers its error.
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
	resp := s.elementProperties(reader, req)
	resp.ExcludedDocuments = reader.exclusions
	return resp, nil
}

// elementProperties answers req over the documents the model is read with.
func (s *Service) elementProperties(reader *elementReader, req *pb.GetElementPropertiesRequest) *pb.GetElementPropertiesResponse {
	if reader.idsRefusal != nil {
		return &pb.GetElementPropertiesResponse{Error: reader.idsRefusal.Error()}
	}
	sym, ok := s.lookupElement(reader, req)
	if ok && !reader.model.Index.Library(sym) {
		if excluded := reader.excluded[sym.DocName]; excluded != nil {
			return &pb.GetElementPropertiesResponse{Error: excluded.Error}
		}
		if sym, ok = reader.held(sym); ok && !reader.model.Index.Library(sym) {
			return reader.read(sym, req.Properties)
		}
	}
	id := req.ElementId
	if ok && sym.Named() {
		id = reader.model.Index.GetFQN(sym)
	}
	return libraryElementProperties(id, req.Properties, s.looksUpByName(req))
}

// lookupElement finds the symbol a request names, as the lookup kind it states; without
// a kind, element_id is read as an id first, then as a name.
func (s *Service) lookupElement(r *elementReader, req *pb.GetElementPropertiesRequest) (*symbols.Symbol, bool) {
	if s.capabilities.has(CapabilitySymbolLookupKind) {
		switch req.Lookup {
		case pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME:
			sym, ok, _ := r.lookupNamed(req.ElementId, r.model.ResolveQualifiedMemberName)
			return sym, ok
		case pb.LookupKind_LOOKUP_KIND_ELEMENT_ID:
			return r.lookupID(req.ElementId)
		}
	}
	return r.lookup(req.ElementId)
}

// looksUpByName reports whether the request reads its element by qualified name alone.
func (s *Service) looksUpByName(req *pb.GetElementPropertiesRequest) bool {
	return s.capabilities.has(CapabilitySymbolLookupKind) && req.Lookup == pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME
}

// libraryElementProperties answers a standard-library element from the API
// element form of its own library document.
func libraryElementProperties(id string, properties []string, byName bool) *pb.GetElementPropertiesResponse {
	element, form, known, err := libraryElementForm(id, byName)
	if err != nil {
		return &pb.GetElementPropertiesResponse{Error: err.Error()}
	}
	if !known {
		return &pb.GetElementPropertiesResponse{Error: fmt.Sprintf("element not found: %s", id)}
	}
	resp := &pb.GetElementPropertiesResponse{Element: libraryElementRefOf(form, element.ID)}
	if len(properties) == 0 {
		for _, property := range element.Properties {
			resp.Properties = append(resp.Properties, libraryElementPropertyOf(form, element, property.Name))
		}
		return resp
	}
	for _, name := range properties {
		resp.Properties = append(resp.Properties, libraryElementPropertyOf(form, element, name))
	}
	return resp
}

// libraryElementForm reads a standard-library element by qualified name alone (byName),
// else by the form's id or the name GetSymbol reports it under.
func libraryElementForm(id string, byName bool) (export.APIJSONElement, *export.ElementForm, bool, error) {
	if byName {
		return export.LibraryElementFormNamed(id)
	}
	return export.LibraryElementForm(id)
}

// elementReader evaluates the metamodel over a model's symbols. The semantic
// model and the evaluator memoize without locking, so reads take mu.
type elementReader struct {
	model *CachedModel
	// exclusions are the documents the model is read without, in document order; excluded indexes them by name.
	exclusions []*pb.ExcludedDocument
	excluded   map[string]*pb.ExcludedDocument
	mu         sync.Mutex
	semantics  *semantics.Model
	// catalog fixes the ids of the bundled libraries' elements and memberships,
	// built on first use.
	catalog   *identity.Catalog
	structure *metamodel.SymbolStructure
	evaluator *metamodel.Evaluator
	// ids spells every element by the id the API element form (Convert) writes and resolves it
	// back; nil when the form refuses the model whole, which every read then answers (idsRefusal).
	ids        *export.SymbolIDs
	idsRefusal error
	addresses  map[*symbols.Symbol]string
	bySymbol   map[string]*symbols.Symbol
	// anonymous keeps what anonymousMember derived, a refusal included.
	anonymous map[*symbols.Symbol]anonymousLookup
}

type anonymousLookup struct {
	member identity.AnonymousMember
	ok     bool
}

// elementReader is the model's symbol-backed property reader, built on first use.
func (m *CachedModel) elementReader() *elementReader {
	m.elementReaderOnce.Do(func() {
		resolver := resolve.New(m.Index)
		// References are read through the resolver's record of the documents (referencedSymbol), made
		// with the semantic model attached so what a redefinition masks (KerML 8.3.3.3.6) is not inherited by name.
		sem := semantics.NewModel(resolver)
		resolver.SetModel(sem)
		for _, doc := range m.Documents {
			if doc != nil && doc.Source != nil && doc.Root != nil {
				resolver.ResolveDocument(doc.Source.Name(), doc.Root)
			}
		}
		sem.SetSourceText(cachedSourceText(m))
		structure := metamodel.NewSymbolStructure(resolver, sem)
		reader := &elementReader{
			model:     m,
			excluded:  map[string]*pb.ExcludedDocument{},
			semantics: sem,
			structure: structure,
			evaluator: metamodel.New(resolver, sem, metamodel.Options{Structure: structure}),
			addresses: map[*symbols.Symbol]string{},
			bySymbol:  map[string]*symbols.Symbol{},
		}
		var excluded []export.ExcludedDocument
		reader.ids, excluded, reader.idsRefusal = m.symbolIDs(resolver, sem)
		for _, exclusion := range excluded {
			left := &pb.ExcludedDocument{Document: exclusion.Document, Error: exclusion.Err.Error()}
			reader.exclusions = append(reader.exclusions, left)
			reader.excluded[left.Document] = left
		}
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

// symbolIDs spells the model's symbols by the ids Convert writes for the documents read, and lists those left out: one the
// parser could not read whole, one whose ids cannot be minted or an earlier document declares; the error refuses the model whole.
func (m *CachedModel) symbolIDs(resolver *resolve.Resolver, sem *semantics.Model) (*export.SymbolIDs, []export.ExcludedDocument, error) {
	unreadable := map[string]error{}
	documents := make([]export.ModelDocument, 0, len(m.Documents))
	for _, doc := range m.Documents {
		if doc == nil || doc.Source == nil || doc.Root == nil {
			continue
		}
		if syntax := convert.SyntaxErrorOf(doc.Source.Name(), doc.Source, doc.ParseDiags); syntax != nil {
			unreadable[doc.Source.Name()] = syntax
		}
		documents = append(documents, export.ModelDocument{File: doc.Source, Root: doc.Root})
	}
	if len(documents) == 0 {
		return nil, nil, nil
	}
	ids, excluded, err := export.ReadableElementIDs(documents, unreadable)
	if err != nil {
		return nil, nil, err
	}
	return export.NewSymbolIDs(ids, resolver, sem), excluded, nil
}

// readable is the first of syms a document the model is read with declares, else the first.
func (r *elementReader) readable(syms []*symbols.Symbol) *symbols.Symbol {
	for _, sym := range syms {
		if sym != nil && r.excluded[sym.DocName] == nil {
			return sym
		}
	}
	return syms[0]
}

// lookup finds the symbol a request addresses: the form's id, a qualified name from the root
// (ResolveMemberName), a symbol id the model resolves (inherited or positional), or a path address.
func (r *elementReader) lookup(id string) (*symbols.Symbol, bool) {
	if sym, ok := r.ids.Symbol(id); ok {
		return sym, true
	}
	if sym, ok, named := r.lookupNamed(id, r.model.ResolveMemberName); named {
		return sym, ok
	}
	if syms := r.model.LookupSymbol(id); len(syms) > 0 {
		return r.readable(syms), true
	}
	return r.lookupPath(id)
}

// lookupNamed finds the element a name states: resolved from the root by resolve, else as a member
// chain names it; named reports the name settled it (declared but not visible from the root: nothing).
func (r *elementReader) lookupNamed(name string, resolve func(string) []*symbols.Symbol) (sym *symbols.Symbol, ok, named bool) {
	if syms := resolve(name); len(syms) > 0 {
		sym := r.readable(syms)
		if r.excluded[sym.DocName] != nil {
			// A name a readable document declares too is read from that one.
			declared := append(symbolfacts.LookupNamed(r.model.Index, name), r.model.LookupNamedMember(name)...)
			if len(declared) > 0 {
				sym = r.readable(append(declared, sym))
			}
		}
		return sym, true, true
	}
	if len(symbolfacts.LookupNamed(r.model.Index, name)) > 0 {
		return nil, false, true
	}
	if syms := r.model.LookupNamedMember(name); len(syms) > 0 {
		return r.readable(syms), true, true
	}
	if parameter := r.statedParameter(name); parameter != nil {
		return parameter, true, true
	}
	return nil, false, false
}

// statedParameter is the parameter an action's notation states under name where no member lookup reaches
// it: a `for` loop's variable, declared in the loop's body, or an assignment's target.
func (r *elementReader) statedParameter(name string) *symbols.Symbol {
	segments := symbolfacts.NameSegments(name)
	if len(segments) < 2 {
		return nil
	}
	loop, variable := segments[len(segments)-2], segments[len(segments)-1]
	// The loop is reached as a name reaches any member: declared, or inherited by the type the path names.
	path := strings.Join(segments[:len(segments)-1], "::")
	owners := r.model.Index.LookupQualified(path)
	if len(owners) == 0 {
		owners = r.model.LookupNamedMember(path)
	}
	if len(owners) == 0 {
		// A segment that holds `::` is one segment only in the notation's spelling.
		quoted := make([]string, len(segments)-1)
		for i, segment := range segments[:len(segments)-1] {
			quoted[i] = identity.Segment(segment)
		}
		owners = r.model.ResolveName(strings.Join(quoted, "::"))
	}
	if len(owners) != 1 || r.model.Index.Library(owners[0]) {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// The member walk splits a segment that holds `::`: the loop found must be the one the path names.
	if !answersTo(r.semantics.NameSourceOf(owners[0]), loop) {
		return nil
	}
	parameters, _ := r.semantics.ReflectiveElements(owners[0], "parameter")
	for _, parameter := range parameters {
		if answersTo(r.semantics.NameSourceOf(parameter), variable) {
			return parameter
		}
	}
	return nil
}

// answersTo reports whether name is the name or the short name source declares.
func answersTo(source *symbols.Symbol, name string) bool {
	return source != nil && (source.Named() && source.Name == name || source.ShortName != "" && source.ShortName == name)
}

// lookupID finds the element an id names: the id the API element form writes, else the id
// GetSymbol reports the element under, else a path address.
func (r *elementReader) lookupID(id string) (*symbols.Symbol, bool) {
	if sym, ok := r.ids.Symbol(id); ok {
		return sym, true
	}
	if syms := r.model.LookupElementID(id); len(syms) > 0 {
		return r.readable(syms), true
	}
	return r.lookupPath(id)
}

// lookupPath resolves a path address (pathSymbol) under r.mu.
func (r *elementReader) lookupPath(id string) (*symbols.Symbol, bool) {
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

// held is the symbol the reader's own model holds for the element sym stands for. A name resolves through the
// symbol context's model, which synthesizes apart; a parameter no element of the form stands for reads as
// the feature of its action's kind it redefines, false when it redefines none.
func (r *elementReader) held(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for range maxPathDepth {
		if sym.Expr == nil && sym.Named() {
			return sym, true
		}
		sym = r.semantics.OwnNodeParameter(sym)
		stated := r.semantics.NamedByNodeParameterRole(sym)
		if id, ok := r.ids.Of(sym); ok {
			// A parameter the form holds no element for shares its span, and so its id, with the expression it is bound to.
			if held, ok := r.ids.Symbol(id); ok && !(stated && sym.Expr != nil && (held.Expr == nil || *held.Expr != *sym.Expr)) {
				return held, true
			}
		}
		if !stated {
			return sym, true
		}
		redefined := r.semantics.ImpliedRedefinitions(sym)
		if len(redefined) == 0 {
			return sym, false
		}
		sym = redefined[0]
	}
	return sym, false
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
	ruleClass := ruleDefiningClass(class, definition)
	if value, ok := r.evaluator.PropertyReadOnly(element, ruleClass, name); ok {
		out.Values = r.values(value)
		return out
	}
	if value, ok := r.statedProperty(element, definition); ok {
		out.Values = r.values(value)
		return out
	}
	if reason, omitted := omittedPropertyReasons()[definition.QualifiedName()]; omitted {
		out.Unsupported = fmt.Sprintf("%s is not derived by OpenSysML: %s", definition.QualifiedName(), reason)
		return out
	}
	if reason, explained := r.evaluator.Explain(element, ruleClass, name); explained {
		out.Unsupported = fmt.Sprintf("%s is not derived for this element: %s", definition.QualifiedName(), reason)
	}
	return out
}

// ruleDefiningClass is the class the evaluator keys the property's rule by: the most specific
// served rule the metaclass specializes (FeatureChainExpression::operator), else the ontology's.
func ruleDefiningClass(metaclass string, definition ontology.Property) string {
	chosen := definition.DefiningClass
	for _, class := range ruleClassesByProperty()[definition.Name] {
		if class != chosen && ontology.IsAncestorOrSelf(metaclass, class) && ontology.IsAncestorOrSelf(class, chosen) {
			chosen = class
		}
	}
	return chosen
}

// ruleClassesByProperty lists, per property name, the classes the served rules are keyed by.
var ruleClassesByProperty = sync.OnceValue(func() map[string][]string {
	classes := map[string][]string{}
	for _, rule := range metamodel.Rules() {
		classes[rule.Property] = append(classes[rule.Property], rule.DefiningClass)
	}
	return classes
})

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

// libraryMembershipRef identifies a library membership, an alias included, by the id the library
// document's form writes for it (fixed for a catalogued member, minted for an unnamed one).
func (r *elementReader) libraryMembershipRef(element metamodel.Element) (*pb.ElementRef, bool) {
	membership := element.Membership
	if !element.IsMembership || membership.Member == nil || !r.model.Index.Library(membership.Member) {
		return nil, false
	}
	ref := &pb.ElementRef{Metaclass: membership.Metaclass()}
	if membership.Symbol != nil {
		// A library alias is the Membership itself.
		alias, ok := identity.AliasMembershipOf(r.semantics, r.libraryCatalog(), membership.Symbol)
		if !ok {
			return nil, false
		}
		ref.Id = alias.ID
		return ref, true
	}
	if library, ok := r.libraryCatalog().ElementForSymbol(membership.Member); ok && library != nil {
		ref.Id = library.OwningMembershipID
	} else if member, ok := r.anonymousMember(membership.Member); ok {
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

// libraryElementID is the id the library document's form writes for sym when no identity names
// it: minted under its catalogued owner for an unnamed member, else its qualified name.
func (r *elementReader) libraryElementID(sym *symbols.Symbol) string {
	if sym.Name == "" {
		// Named only by redefinition (BooleanEvaluation's `return : Boolean;`):
		// the catalog keys it by its effective name.
		if element, ok := r.libraryCatalog().ElementForSymbol(sym); ok && element != nil && element.ID != "" {
			return element.ID
		}
		if member, ok := r.anonymousMember(sym); ok {
			return member.ID
		}
		return r.semantics.QualifiedNameOf(sym)
	}
	return r.model.Index.GetFQN(sym)
}

// anonymousMember is identity.AnonymousMemberOf, derived once per symbol: placing a member walks
// its owner's members, and every reference to an unnamed library member asks again.
func (r *elementReader) anonymousMember(sym *symbols.Symbol) (identity.AnonymousMember, bool) {
	if found, ok := r.anonymous[sym]; ok {
		return found.member, found.ok
	}
	member, ok := identity.AnonymousMemberOf(r.semantics, r.libraryCatalog(), sym)
	if r.anonymous == nil {
		r.anonymous = map[*symbols.Symbol]anonymousLookup{}
	}
	r.anonymous[sym] = anonymousLookup{member: member, ok: ok}
	return member, ok
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
		if sym.Name == "" {
			return r.libraryEffectiveQualifiedName(sym)
		}
		return r.model.Index.GetFQN(sym)
	}
	// Spelled by the metamodel's own Element::qualifiedName (KerML 8.2.4), the rule the export
	// writes too: a segment that is no identifier is quoted (Cmd's conjugate is P::Cmd::'~Cmd').
	if value, ok := r.evaluator.PropertyReadOnly(r.structure.ElementFor(sym), "Element", "qualifiedName"); ok && value.Kind == metamodel.StringValue {
		return value.String
	}
	return r.model.Index.QualifiedIdentity(sym)
}

// libraryEffectiveQualifiedName names an unnamed library feature by its effective name (KerML 8.2.4;
// BooleanEvaluation's `return : Boolean;` is Performances::BooleanEvaluation::result), else by its library path.
func (r *elementReader) libraryEffectiveQualifiedName(sym *symbols.Symbol) string {
	if r.semantics == nil {
		return ""
	}
	name := r.semantics.EffectiveNameOf(sym)
	owner := sym.Owner()
	if name == "" || owner == nil {
		if member, ok := r.anonymousMember(sym); ok {
			return member.Path
		}
		return ""
	}
	ownerName := r.model.Index.GetFQN(owner)
	if ownerName == "" {
		return ""
	}
	return ownerName + "::" + name
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
