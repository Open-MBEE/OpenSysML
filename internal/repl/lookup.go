package repl

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// lookupSymbol resolves the name a REPL command was given against the session
// document. It is the single lookup path for every symbol-taking command, so
// `%instantiate`, `%eval`, `%calc` and the debuggers all agree on what a name
// denotes.
//
// A simple name (Vehicle) is searched through the whole scope tree, so a member
// of a package is found without qualification; a qualified name
// (Demo::Vehicle) is resolved through the symbol index, the same API the gRPC
// service resolves its symbol ids with. The second result is the
// fully-qualified name the symbol was found under, which is what instances are
// keyed by so the spelling used to create one need not be the spelling used to
// inspect it.
//
// The name is taken in the notation, so a segment quoted because it holds a
// space or is a keyword ('My Pkg'::Car) denotes what the index registers it as.
func (s *Session) lookupSymbol(name string) (*symbols.Symbol, string, error) {
	return s.lookupSymbolOfKinds(name)
}

// lookupSymbolOfKinds resolves a name as lookupSymbol does. want narrows the
// suggestions offered when nothing resolves to the kinds the command can act
// on, and decides nothing about what a resolved name denotes.
func (s *Session) lookupSymbolOfKinds(name string, want ...symbols.SymbolKind) (*symbols.Symbol, string, error) {
	// A name the notation reads is resolved by the text it names; anything else is
	// looked up as typed, so the failure reported is about what was typed.
	qualified := strings.Contains(name, "::")
	if p := s.parseName(name); p.ok {
		name, qualified = p.name, p.qualified
	}
	docScopes := s.docScopes()
	// The library is searched even from an empty session, so a qualified
	// library name is not answered with "no declarations loaded".
	idx := s.browseIndex()
	if idx == nil {
		return nil, "", fmt.Errorf("no declarations loaded")
	}

	if qualified {
		matches := idx.LookupQualified(name)
		switch len(matches) {
		case 0:
			// A name may reach through a declared feature into its type
			// (Outer::inner::b), which no declaration is registered under.
			if sym, fqn := s.featureChainSymbol(name); sym != nil {
				if local := scopeSymbolForAny(docScopes, sym.Decl); local != nil {
					sym = local
				}
				return sym, fqn, nil
			}
			return nil, "", s.notFoundError(name, want...)
		case 1:
			// The index owns its own scope tree; map the hit back onto the
			// document's tree so every command sees one symbol per declaration.
			sym := matches[0]
			fqn := idx.GetFQN(sym)
			if fqn == "" {
				fqn = name
			}
			if local := scopeSymbolForAny(docScopes, sym.Decl); local != nil {
				sym = local
			}
			return sym, fqn, nil
		default:
			return nil, "", ambiguousError(name, matches, idx)
		}
	}

	matches := s.nameTable().lookup(name)
	switch len(matches) {
	case 0:
		// A name the session declares nowhere may still be visible where the
		// prompt evaluates — through an import of that namespace.
		if sym, ok := resolve.New(idx).LookupName(s.promptScope(), name); ok && sym != nil {
			return sym, idx.GetFQN(sym), nil
		}
		// A top-level library name (ISQ) is qualified by nothing, so the index
		// registers it under the name itself.
		if matches := idx.LookupQualified(name); len(matches) == 1 {
			return matches[0], idx.GetFQN(matches[0]), nil
		}
		return nil, "", s.notFoundError(name, want...)
	case 1:
		return matches[0], idx.GetFQN(matches[0]), nil
	default:
		return nil, "", ambiguousError(name, matches, idx)
	}
}

// owningInstance returns the object a fully-qualified name belongs to: the
// object its qualifier names, since the last segment is the feature read from
// it. The second result is the found object's FQN, for reporting.
func (s *Session) owningInstance(fqn string) (*runtime.Instance, string) {
	segments := strings.Split(fqn, "::")
	if len(segments) < 2 {
		return nil, ""
	}
	return s.objectNamed(strings.Join(segments[:len(segments)-1], "::"))
}

// objectNamed returns the object a fully-qualified name denotes: the one
// materialized under it, or the longest instantiated prefix with the remaining
// segments walked through that instance's feature values, since a nested part is an
// object of its own. The second result is the found object's label, for reporting.
func (s *Session) objectNamed(fqn string) (*runtime.Instance, string) {
	if fqn == "" {
		return nil, ""
	}
	segments := strings.Split(fqn, "::")
	for i := len(segments); i > 0; i-- {
		key := strings.Join(segments[:i], "::")
		inst, ok := s.instances[key]
		if !ok {
			continue
		}
		return s.walkFeatureValues(inst, s.declaredName(key), segments[i:])
	}
	return nil, ""
}

// featureChainSymbol resolves a qualified name whose later segments are members
// of the type of the segment before them (Outer::inner::b::c): the index
// registers a declaration only under its own owner, so such a chain is walked
// through the model's member lookup, which follows typing and specialization.
// The second result is the fully-qualified name the symbol is declared under.
func (s *Session) featureChainSymbol(name string) (*symbols.Symbol, string) {
	segments := strings.Split(name, "::")
	if len(segments) < 3 {
		return nil, ""
	}
	idx := s.browseIndex()
	ctx, err := s.getOrCreateRuntime()
	if idx == nil || err != nil {
		return nil, ""
	}
	model := ctx.Model()
	// The longest prefix a declaration answers to is the chain's root, so a
	// nested feature is preferred over the type it happens to share a name with.
	for i := len(segments) - 1; i > 0; i-- {
		matches := idx.LookupQualified(strings.Join(segments[:i], "::"))
		if len(matches) != 1 {
			continue
		}
		sym := matches[0]
		walked := true
		for _, seg := range segments[i:] {
			member, ok := model.LookupMember(sym, seg)
			if !ok || member == nil {
				walked = false
				break
			}
			sym = member
		}
		if !walked {
			continue
		}
		fqn := idx.GetFQN(sym)
		if fqn == "" {
			fqn = name
		}
		return sym, fqn
	}
	return nil, ""
}

// carrierLimit bounds the object graph a subject is searched through, so a
// deeply nested or richly connected model cannot make a lookup unbounded.
const carrierLimit = 2000

// carriesDeclaration reports whether an object of type typ carries the feature
// declared by decl: typ or a supertype of it is that declaration. Declarations
// are compared, not symbols, because the index and the document each build a
// symbol of their own for one declaration.
func carriesDeclaration(model *semantics.Model, typ *symbols.Symbol, decl ast.Node) bool {
	if typ == nil || decl == nil {
		return false
	}
	if typ.Decl == decl {
		return true
	}
	for _, sup := range model.AllSupertypes(typ) {
		if sup != nil && sup.Decl == decl {
			return true
		}
	}
	return false
}

// carrierInstances labels the session's objects of the type declaring sym,
// sorted: an object of `part hot : Sensor` carries `Sensor::inRange`. Nested
// objects carry the features of their own type too, so `Spec::c` is carried by
// the `o::inner::b` a redefinition gave a value on, not only by a top-level
// object. An object displaced from its name carries under its id, after the
// named ones, so an object two closures share is named the way it is still
// addressed by name.
func (s *Session) carrierInstances(sym *symbols.Symbol) []string {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	declaring := sym.OwnerScope.Owner()
	if declaring == nil || declaring.Decl == nil || s.heldObjects() == 0 {
		return nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil
	}
	model := ctx.Model()
	var names []string
	// A feature is read from the outermost object carrying it; its own nested
	// objects are of other types and are not searched again.
	s.walkObjects(nestedObjectsIn(ctx), func(cur carrier) bool {
		if !carriesDeclaration(model, cur.inst.Type, declaring.Decl) {
			return true
		}
		names = append(names, cur.name)
		return false
	})
	sort.SliceStable(names, func(i, j int) bool { return carrierLess(names[i], names[j]) })
	return names
}

// walkObjects visits the session's objects and, while visit reports true, the
// objects nested yields for them, breadth-first in carrierLess order and within
// carrierLimit.
func (s *Session) walkObjects(nested func(carrier) []carrier, visit func(carrier) bool) {
	s.walk(nested, visit, carrierLimit)
}

// walkHeldObjects visits every object the session holds: its roots and, while
// visit reports true, what their materialized feature values hold. Nothing is
// materialized, so the walk ends with the objects that exist and needs no bound.
func (s *Session) walkHeldObjects(ctx *runtime.Context, visit func(carrier) bool) {
	s.walk(materializedObjectsIn(ctx), visit, 0)
}

// walk is walkObjects within limit objects visited, or unbounded for a limit of 0.
func (s *Session) walk(nested func(carrier) []carrier, visit func(carrier) bool, limit int) {
	seen := make(map[int64]bool, s.heldObjects())
	queue := s.rootCarriers()
	for visited := 0; len(queue) > 0 && (limit == 0 || visited < limit); visited++ {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur.inst.ID] {
			continue
		}
		seen[cur.inst.ID] = true
		if visit(cur) {
			queue = append(queue, nested(cur)...)
		}
	}
}

// carrier is an object reachable from the session's objects, under the label
// it is reached by.
type carrier struct {
	name string
	inst *runtime.Instance
}

// rootCarriers is every object the session holds: the named ones by name, then
// the displaced ones as `#<id>`, in carrierLess order.
func (s *Session) rootCarriers() []carrier {
	roots := make([]carrier, 0, s.heldObjects())
	for name, inst := range s.instances {
		if inst != nil {
			roots = append(roots, carrier{name: s.declaredName(name), inst: inst})
		}
	}
	for _, u := range s.unnamed {
		if u.obj != nil {
			roots = append(roots, carrier{name: fmt.Sprintf("#%d", u.obj.ID), inst: u.obj})
		}
	}
	sort.Slice(roots, func(i, j int) bool { return carrierLess(roots[i].name, roots[j].name) })
	return roots
}

// carrierLess orders labels: named objects alphabetically before displaced
// ones, which go by id; indexed elements go by index, so wheels[2] precedes
// wheels[10].
func carrierLess(a, b string) bool {
	aRef, bRef := parseLabel(a), parseLabel(b)
	switch {
	case (aRef.id > 0) != (bRef.id > 0):
		return aRef.id == 0
	case aRef.id != bRef.id:
		return aRef.id < bRef.id
	}
	for i := 0; i < len(aRef.segments) && i < len(bRef.segments); i++ {
		aSeg, bSeg := aRef.segments[i], bRef.segments[i]
		if aSeg.name != bSeg.name {
			return aSeg.name < bSeg.name
		}
		if aSeg.index != bSeg.index {
			return aSeg.index < bSeg.index
		}
	}
	return len(aRef.segments) < len(bRef.segments)
}

// parseLabel reads a label back as the reference it spells. A label is built
// from names the notation quotes as needed, so it always reads; text that does
// not is one nameless segment, ordered by its spelling.
func parseLabel(label string) objectRef {
	ref, err := parseObjectRef(label)
	if err != nil {
		return objectRef{text: label, segments: []objectSegment{{text: label, name: label}}}
	}
	return ref
}

// carrierObject is the object a carrier label denotes, and the label it is
// reported under.
func (s *Session) carrierObject(label string) (*runtime.Instance, string) {
	inst, owner, err := s.resolveObject(label)
	if err != nil {
		return nil, ""
	}
	return inst, owner
}

// nestedObjectsIn yields the objects held in an object's feature values. A part
// feature value holds its object only once it is asked for, so asking
// materializes it.
func nestedObjectsIn(ctx *runtime.Context) func(carrier) []carrier {
	return func(of carrier) []carrier {
		return nestedObjects(ctx, of, func(name string) (*runtime.FeatureValue, bool) {
			fv, err := of.inst.GetFeatureValue(ctx, name)
			return fv, err == nil && fv != nil
		})
	}
}

// heldByID finds the object with id among those the session holds, materializing
// nothing; keeper is the held object that kept id for a connector set aside by a carry-over.
func (s *Session) heldByID(id int64) (found, keeper *runtime.Instance) {
	s.walkHeldObjects(s.rtCtx, func(cur carrier) bool {
		if found != nil {
			return false
		}
		if cur.inst.ID == id {
			found = cur.inst
		} else if keeper == nil && slices.Contains(cur.inst.KeptConnectorIDs(), id) {
			keeper = cur.inst
		}
		return found == nil
	})
	return found, keeper
}

// heldIDs lists the ids of the objects the session holds, ascending, the
// connectors a carry-over set aside included.
func (s *Session) heldIDs() []int64 {
	var ids []int64
	s.walkHeldObjects(s.rtCtx, func(cur carrier) bool {
		ids = append(ids, cur.inst.ID)
		ids = append(ids, cur.inst.KeptConnectorIDs()...)
		return true
	})
	slices.Sort(ids)
	return ids
}

// materializedObjectsIn yields only the objects an object already holds — in its
// materialized feature values, and its anonymous connectors once `%features` has
// shown them, reachable by id alone — so a walk leaves the runtime as it found it.
func materializedObjectsIn(ctx *runtime.Context) func(carrier) []carrier {
	return func(of carrier) []carrier {
		out := nestedObjects(ctx, of, func(name string) (*runtime.FeatureValue, bool) {
			fv := of.inst.FeatureValues[name]
			return fv, fv != nil && fv.Materialized
		})
		for _, conn := range of.inst.MaterializedConnectors(ctx) {
			out = append(out, carrier{name: fmt.Sprintf("#%d", conn.ID), inst: conn})
		}
		return out
	}
}

// nestedObjects returns the objects held in the feature values read yields, each
// once, in feature-value-name order, under the label it is reached by: the first
// single-valued feature holding it, else its 1-based place in a multi-valued
// one, `.wheels[2]`.
func nestedObjects(ctx *runtime.Context, of carrier, read func(string) (*runtime.FeatureValue, bool)) []carrier {
	fvs := make([]string, 0, len(of.inst.FeatureValues))
	for name := range of.inst.FeatureValues {
		fvs = append(fvs, name)
	}
	sort.Strings(fvs)
	type held struct {
		child   *runtime.Instance
		segment string
		indexed bool
	}
	var order []int64
	reached := make(map[int64]*held)
	reach := func(val runtime.Value, segment string, indexed bool) {
		id, isObject := val.Object()
		if !isObject {
			return
		}
		child, ok := ctx.Instance(id)
		if !ok || child == nil {
			return
		}
		h := reached[id]
		if h == nil {
			h = &held{child: child, segment: segment, indexed: indexed}
			reached[id] = h
			order = append(order, id)
		} else if h.indexed && !indexed {
			h.segment, h.indexed = segment, false
		}
	}
	for _, name := range fvs {
		fv, ok := read(name)
		if !ok {
			continue
		}
		if fv.Values.Kind == runtime.ValInvalid {
			reach(fv.Value, lexer.NameText(name), false)
			continue
		}
		for i, val := range collectionElements(fv.Values) {
			reach(val, fmt.Sprintf("%s[%d]", lexer.NameText(name), i+1), true)
		}
	}
	out := make([]carrier, 0, len(order))
	for _, id := range order {
		h := reached[id]
		out = append(out, carrier{name: of.name + "." + h.segment, inst: h.child})
	}
	return out
}

// AmbiguousSubjectError reports a feature or condition several of the session's
// objects carry, so which one an answer would be about is a question.
type AmbiguousSubjectError struct {
	Name     string
	Carriers []string
}

func (e *AmbiguousSubjectError) Error() string {
	return fmt.Sprintf("%s is carried by more than one object of this session (%s): name one of them, or start a session holding the one you mean",
		e.Name, strings.Join(e.Carriers, ", "))
}

// subjectFor is the object an answer about sym is about: the one
// instantiated under the name it was reached by, else the single carrier. Several
// carriers are an AmbiguousSubjectError; none leaves the answer about declared
// defaults. name is the spelling to report, fqn the resolved one.
func (s *Session) subjectFor(name, fqn string, sym *symbols.Symbol) (*runtime.Instance, string, error) {
	if inst, owner := s.owningInstance(fqn); inst != nil {
		return inst, owner, nil
	}
	carriers := s.carrierInstances(sym)
	switch len(carriers) {
	case 0:
		return nil, "", nil
	case 1:
		// A carrier may be nested, so it is reached the way any object name is.
		inst, owner := s.carrierObject(carriers[0])
		if inst == nil {
			return nil, "", nil
		}
		return inst, owner, nil
	default:
		return nil, "", &AmbiguousSubjectError{Name: name, Carriers: carriers}
	}
}

// walkFeatureValues follows a chain of feature names from inst, labelled label.
// An unwalkable segment yields no object, since binding to an ancestor would
// answer about the wrong one.
func (s *Session) walkFeatureValues(inst *runtime.Instance, label string, names []string) (*runtime.Instance, string) {
	if len(names) == 0 {
		return inst, label
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, ""
	}
	path := make([]objectSegment, 0, len(names))
	for _, name := range names {
		path = append(path, objectSegment{text: lexer.NameText(name), name: name})
	}
	inst, label, err = s.walkObjectPath(ctx, inst, label, path)
	if err != nil {
		return nil, ""
	}
	return inst, label
}

// An object reference is how every command that takes an object names one:
//
//	#<id>                     the id %instantiate printed (#3)
//	<name>                    a declared name an object was created under (car, Demo::car)
//	<object>.<feature>...     the object a feature of it holds (car.fl, #3.fl.hub)
//	<object>.<feature>[<n>]   the n-th object of a multi-valued feature, counted from 1
//
// `.` and `::` separate segments alike, so `Demo::car::fl` and `Demo::car.fl` name
// one object. The leading segments are the declared name — the longest run of them
// a declaration answers to and an object was created under — and the rest are
// features walked through that object's feature values.
//
// The REPL reports an object under a label: its declared name or id with the
// walked features each after a `.`, every name spelled as the notation writes
// it. A label is thus a reference that reads back to the object — a `.` segment
// is only ever a feature, so no declaration can capture it — and one a name
// alone cannot forge: an object named '#3' or a feature named 'wheels[2]' is
// quoted, where a generated id or index is not.

// objectSegment is one segment of an object reference.
type objectSegment struct {
	text   string // as typed, quotes kept, so a lookup reads the notation
	name   string // the declaration or feature it names
	dotted bool   // written after a `.` rather than `::`
	index  int    // 1-based element of a multi-valued feature, 0 for none
}

// objectRef is a parsed object reference.
type objectRef struct {
	text     string
	id       int64 // the root object's id, 0 when the root is a declared name
	segments []objectSegment
}

// ObjectRefError reports text that is no object reference at all.
type ObjectRefError struct {
	Ref    string
	Detail string
	// Named is the `::`-joined run of names read before a character no unquoted
	// name holds stopped the text: `T::SA` in `T::SA-506`.
	Named string
	// Hint is what Named may have meant, appended to the report.
	Hint string
}

func (e *ObjectRefError) Error() string {
	return fmt.Sprintf("%q is not an object reference: %s%s", e.Ref, e.Detail, e.Hint)
}

// UnknownObjectIDError reports an id no object of the session has, with the
// ids that do exist so the reader can pick one.
type UnknownObjectIDError struct {
	ID    int64
	Known []int64
	// Err is what kept the connector a carry-over set aside under ID from being
	// materialized again, nil when no object was ever there.
	Err error
}

// unknownIDListed bounds the ids an UnknownObjectIDError spells out.
const unknownIDListed = 20

func (e *UnknownObjectIDError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("no object #%d in this session: the connector that had that identity cannot be materialized again: %v", e.ID, e.Err)
	}
	if len(e.Known) == 0 {
		return fmt.Sprintf("no object #%d in this session: nothing materialized has that identity (no objects have been created)", e.ID)
	}
	listed := e.Known
	more := ""
	if len(listed) > unknownIDListed {
		listed = listed[:unknownIDListed]
		more = fmt.Sprintf(" … (%d in all)", len(e.Known))
	}
	ids := make([]string, len(listed))
	for i, id := range listed {
		ids[i] = fmt.Sprintf("#%d", id)
	}
	return fmt.Sprintf("no object #%d in this session: nothing materialized has that identity (the objects are %s%s)", e.ID, strings.Join(ids, ", "), more)
}

func (e *UnknownObjectIDError) Unwrap() error { return e.Err }

// NotInstantiatedError reports a declared name no object has been created
// under. When objects of the definition that name is typed by (or, for a
// definition, objects of usages typed by it) exist, it says so and names what
// to instantiate.
type NotInstantiatedError struct {
	Name string // the name asked for, as the prompt prints names
	// Definition is the definition the objects in Objects are of, "" when none.
	Definition string
	// Objects are the related objects, in carrier order.
	Objects []RelatedObject
	// UsageAsked reports that Name is a usage, so Objects are of its definition
	// rather than of the usage; otherwise Name is a definition Objects are typed by.
	UsageAsked bool
}

// RelatedObject names an object the session holds, by id and by the label it is
// reached under — "" for one only its id reaches.
type RelatedObject struct {
	ID    int64
	Label string
}

// relatedObjectsListed bounds the objects a NotInstantiatedError spells out.
const relatedObjectsListed = 5

func (e *NotInstantiatedError) Error() string {
	if len(e.Objects) == 0 {
		return fmt.Sprintf("no instance of %q (use %%instantiate first)", e.Name)
	}
	listed := e.Objects
	more := ""
	if len(listed) > relatedObjectsListed {
		listed = listed[:relatedObjectsListed]
		more = fmt.Sprintf(" … (%d in all)", len(e.Objects))
	}
	mentions := make([]string, len(listed))
	labels := make([]string, len(listed))
	for i, o := range listed {
		mentions[i] = fmt.Sprintf("#%d", o.ID)
		labels[i] = mentions[i]
		if o.Label != "" {
			mentions[i] = fmt.Sprintf("#%d of %q", o.ID, o.Label)
			labels[i] = o.Label
		}
	}
	objects, is := "object "+mentions[0]+" is", "it"
	if len(e.Objects) > 1 {
		objects, is = "objects "+strings.Join(mentions, ", ")+more+" are", "one of them"
	}
	if e.UsageAsked {
		return fmt.Sprintf("no instance of the usage %q: %s of its definition %q, not of the usage — use %%instantiate %s to create the usage's object, or name %s to address %s",
			e.Name, objects, e.Definition, e.Name, strings.Join(labels, " or "), is)
	}
	return fmt.Sprintf("no instance of the definition %q itself: %s typed by it — name %s to address %s, or use %%instantiate %s to create an object of the definition",
		e.Name, objects, strings.Join(labels, " or "), is, e.Name)
}

// notInstantiated reports that nothing is materialized under fqn, declared by
// sym, and — when objects of the definition the name is typed by, or typed by
// the definition it names, exist — what to instantiate instead.
func (s *Session) notInstantiated(sym *symbols.Symbol, fqn string) error {
	e := &NotInstantiatedError{Name: s.declaredName(fqn)}
	if sym == nil || sym.Decl == nil || s.heldObjects() == 0 {
		return e
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return e
	}
	model := ctx.Model()
	var definition *symbols.Symbol
	switch sym.Decl.(type) {
	case *ast.Usage:
		e.UsageAsked = true
		for _, sup := range model.AllSupertypes(sym) {
			if _, isDef := sup.Decl.(*ast.Definition); isDef {
				definition = sup
				break
			}
		}
	case *ast.Definition:
		definition = sym
	}
	if definition == nil || definition.Decl == nil {
		return e
	}
	e.Definition = declarationNotation(definition)
	// An error names what exists; it materializes nothing to find it.
	s.walkHeldObjects(ctx, func(cur carrier) bool {
		if carriesDeclaration(model, cur.inst.Type, definition.Decl) {
			related := RelatedObject{ID: cur.inst.ID}
			if !isObjectID(cur.name) {
				related.Label = cur.name
			}
			e.Objects = append(e.Objects, related)
		}
		return true
	})
	return e
}

// ObjectPathError reports a segment of an object reference that names no object
// of the one before it: Object is that object as the REPL reports it, Segment
// the offending segment as typed.
type ObjectPathError struct {
	Object  string
	Segment string
	Detail  string
	// Err is what kept the segment's feature value from materializing, nil when
	// the segment reached a value that is no object or no feature at all.
	Err error
}

func (e *ObjectPathError) Error() string {
	return e.Detail
}

func (e *ObjectPathError) Unwrap() error { return e.Err }

// pathError builds an ObjectPathError about seg read from the object labelled object.
func pathError(object string, seg objectSegment, format string, args ...any) *ObjectPathError {
	return &ObjectPathError{Object: object, Segment: seg.text, Detail: fmt.Sprintf(format, args...)}
}

// isObjectID reports whether text is an id alone, `#` followed by digits.
func isObjectID(text string) bool {
	return len(text) > 1 && text[0] == '#' && leadingDigits(text[1:]) == text[1:]
}

// leadingDigits returns the run of ASCII digits text starts with.
func leadingDigits(text string) string {
	i := 0
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	return text[:i]
}

// looksLikeObjectPath reports whether text can only be an object reference — it
// starts with an id or walks a feature with `.` or an index — so a failure to
// resolve it is reported as such rather than tried as a declaration's name.
func looksLikeObjectPath(text string) bool {
	if strings.HasPrefix(text, "#") {
		return true
	}
	inName, escaped := false, false
	for _, r := range text {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inName = !inName
		case !inName && (r == '.' || r == '['):
			return true
		}
	}
	return false
}

// parseObjectRef reads an object reference, reporting what makes text none.
func parseObjectRef(text string) (objectRef, error) {
	ref := objectRef{text: text}
	rest := text
	if rest == "" {
		return ref, &ObjectRefError{Ref: text, Detail: "nothing was named"}
	}
	if strings.HasPrefix(rest, "#") {
		digits := leadingDigits(rest[1:])
		if digits == "" {
			return ref, &ObjectRefError{Ref: text, Detail: "an object id is written #<id>, with the number %instantiate printed"}
		}
		id, err := strconv.ParseInt(digits, 10, 64)
		if err != nil || id <= 0 {
			return ref, &ObjectRefError{Ref: text, Detail: fmt.Sprintf("#%s is not an object id (ids count up from 1)", digits)}
		}
		ref.id = id
		rest = rest[1+len(digits):]
		if rest == "" {
			return ref, nil
		}
		var ok bool
		if _, rest, ok = cutSeparator(rest); !ok {
			return ref, &ObjectRefError{Ref: text, Detail: fmt.Sprintf("a feature of #%d is written after . or ::, not %q", id, rest)}
		}
	}
	dotted := false
	for {
		seg, after, err := scanObjectSegment(text, rest)
		if err != nil {
			return ref, err
		}
		seg.dotted = dotted
		if len(ref.segments) == 0 && ref.id == 0 && seg.index > 0 {
			return ref, &ObjectRefError{Ref: text, Detail: fmt.Sprintf("%s takes no index: an index picks an element of a multi-valued feature", seg.text)}
		}
		ref.segments = append(ref.segments, seg)
		if after == "" {
			return ref, nil
		}
		sep, next, ok := cutSeparator(after)
		if !ok {
			err := &ObjectRefError{Ref: text, Detail: fmt.Sprintf("%q cannot follow %s: segments are separated by . or ::", after, seg.text)}
			if ref.id == 0 {
				err.Named = declaredRun(ref.segments)
			}
			return ref, err
		}
		if next == "" {
			return ref, &ObjectRefError{Ref: text, Detail: fmt.Sprintf("it ends in %q with no feature after it", sep)}
		}
		rest, dotted = next, sep == "."
	}
}

// declaredRun is the `::`-joined registered names of segments that may all name
// a declaration — none reached through `.` or an index — or "" when one is not.
func declaredRun(segments []objectSegment) string {
	names := make([]string, len(segments))
	for i, seg := range segments {
		if seg.dotted || seg.index > 0 {
			return ""
		}
		names[i] = seg.name
	}
	return strings.Join(names, "::")
}

// cutSeparator splits the segment separator text starts with from what follows.
func cutSeparator(text string) (sep, rest string, ok bool) {
	switch {
	case strings.HasPrefix(text, "::"):
		return "::", text[2:], true
	case strings.HasPrefix(text, "."):
		return ".", text[1:], true
	}
	return "", text, false
}

// scanObjectSegment reads one segment — a name, quoted or not, and an optional
// index — from the front of rest; ref is the whole reference, for reporting.
func scanObjectSegment(ref, rest string) (objectSegment, string, error) {
	var seg objectSegment
	end := 0
	if strings.HasPrefix(rest, "'") {
		escaped := false
		for i, r := range rest[1:] {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '\'':
				end = i + 2
			}
			if end > 0 {
				break
			}
		}
		if end == 0 {
			return seg, "", &ObjectRefError{Ref: ref, Detail: fmt.Sprintf("the quoted name %s is not closed", rest)}
		}
		plain, ok := plainName(rest[:end])
		if !ok {
			return seg, "", &ObjectRefError{Ref: ref, Detail: fmt.Sprintf("%s is not a name", rest[:end])}
		}
		seg.text, seg.name = rest[:end], plain
	} else {
		for end < len(rest) {
			r, size := utf8.DecodeRuneInString(rest[end:])
			if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
			end += size
		}
		if end == 0 {
			return seg, "", &ObjectRefError{Ref: ref, Detail: fmt.Sprintf("a name was expected at %q", rest)}
		}
		seg.text, seg.name = rest[:end], rest[:end]
	}
	rest = rest[end:]
	if !strings.HasPrefix(rest, "[") {
		return seg, rest, nil
	}
	close := strings.IndexByte(rest, ']')
	if close < 0 {
		return seg, "", &ObjectRefError{Ref: ref, Detail: fmt.Sprintf("the index after %s is not closed with ]", seg.text)}
	}
	digits := rest[1:close]
	index, err := strconv.Atoi(digits)
	if digits == "" || leadingDigits(digits) != digits || err != nil || index < 1 {
		return seg, "", &ObjectRefError{Ref: ref, Detail: fmt.Sprintf("%s[%s] is not an index: elements are counted from 1", seg.text, digits)}
	}
	seg.index = index
	seg.text += rest[:close+1]
	return seg, rest[close+1:], nil
}

// resolveObject is the one path every object-taking command resolves its
// argument through: the object the reference denotes and the label the REPL
// reports it under, or why it denotes none.
func (s *Session) resolveObject(text string) (*runtime.Instance, string, error) {
	ref, err := parseObjectRef(text)
	if err != nil {
		var bad *ObjectRefError
		if errors.As(err, &bad) && bad.Named != "" {
			bad.Hint = suggest.Hint("", bad.Named, nil, s.unquotedNames(bad.Named, nil))
		}
		return nil, "", err
	}
	if ref.id > 0 {
		// No runtime is no object at all, so none is built only to say so.
		if s.rtCtx == nil {
			return nil, "", &UnknownObjectIDError{ID: ref.id}
		}
		inst, keeper := s.heldByID(ref.id)
		if inst == nil && keeper != nil {
			conn, err := keeper.RestoreConnector(s.rtCtx, ref.id)
			if err != nil && conn == nil {
				return nil, "", &UnknownObjectIDError{ID: ref.id, Err: err}
			}
			if err != nil {
				return nil, "", fmt.Errorf("#%d is materialized again, but an older object's behavior failed: %w", ref.id, err)
			}
			inst = conn
		}
		if inst == nil {
			return nil, "", &UnknownObjectIDError{ID: ref.id, Known: s.heldIDs()}
		}
		return s.walkObjectPath(s.rtCtx, inst, fmt.Sprintf("#%d", ref.id), ref.segments)
	}
	return s.resolveNamedObject(ref)
}

// resolveNamedObject resolves a reference rooted at a declared name: the
// longest run of leading segments that names an object is that object, and what
// follows is walked through its feature values. A run that only reaches a
// declaration is reported as an uninstantiated one, and one that reaches nothing
// as the unresolved name it is.
func (s *Session) resolveNamedObject(ref objectRef) (*runtime.Instance, string, error) {
	inst, fqn, rest, err := s.namedRoot(ref)
	if err != nil {
		return nil, "", err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, "", err
	}
	return s.walkObjectPath(ctx, inst, s.declaredName(fqn), rest)
}

// namedRoot finds the object a name-rooted reference starts from: the object,
// its qualified name and the segments left to walk from it.
func (s *Session) namedRoot(ref objectRef) (*runtime.Instance, string, []objectSegment, error) {
	// The declared name is the `::`-joined run before the first `.` or index; a
	// segment after `.` is a feature of the object before it, never a declaration.
	head := len(ref.segments)
	for i, seg := range ref.segments {
		if seg.index > 0 || seg.dotted {
			head = i
			break
		}
	}
	var (
		noInstance    string
		noInstanceSym *symbols.Symbol
		unresolved    error
	)
	for i := head; i > 0; i-- {
		name := joinTyped(ref.segments[:i])
		sym, fqn, err := s.lookupSymbol(name)
		if err != nil {
			var ambiguous *AmbiguousNameError
			if errors.As(err, &ambiguous) {
				return nil, "", nil, err
			}
			if i == head {
				unresolved = err
			}
			continue
		}
		if inst, ok := s.instances[fqn]; ok {
			return inst, fqn, ref.segments[i:], nil
		}
		if i == head && head < len(ref.segments) && isNamespaceSymbol(sym) {
			shown := declarationNotation(sym)
			return nil, "", nil, &ObjectRefError{Ref: ref.text, Detail: fmt.Sprintf("%s is a %s, not an object: its member is written %s::%s", shown, namespaceKind(sym), shown, ref.segments[head].text)}
		}
		if noInstance == "" {
			noInstance, noInstanceSym = fqn, sym
		}
	}
	if noInstance != "" {
		return nil, "", nil, s.notInstantiated(noInstanceSym, noInstance)
	}
	if unresolved == nil {
		_, _, unresolved = s.lookupSymbol(joinTyped(ref.segments[:head]))
	}
	return nil, "", nil, unresolved
}

// isNamespaceSymbol reports whether sym is a package or namespace, which holds
// members but is never an object.
func isNamespaceSymbol(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace)
}

// namespaceKind names what a namespace symbol is for a message.
func namespaceKind(sym *symbols.Symbol) string {
	if sym.Kind == symbols.SymbolPackage {
		return "package"
	}
	return "namespace"
}

// joinTyped spells segments as the qualified name they were typed as.
func joinTyped(segments []objectSegment) string {
	texts := make([]string, len(segments))
	for i, seg := range segments {
		texts[i] = seg.text
	}
	return strings.Join(texts, "::")
}

// heldObject is the object the session holds under a label it reported — a
// declared name, an id or a path — if it still holds one.
func (s *Session) heldObject(label string) (*runtime.Instance, bool) {
	inst, _, err := s.resolveObject(label)
	return inst, err == nil && inst != nil
}

// relabelByID respells a label rooted at the object fqn currently names with the
// object's id, `Obj::Monitor.modes` → `#1.modes`, so the label still reaches it
// once the name denotes another object. Other labels are returned unchanged.
func (s *Session) relabelByID(label, fqn string) string {
	ref := parseLabel(label)
	if ref.id > 0 {
		return label
	}
	inst, root, rest, err := s.namedRoot(ref)
	if err != nil || root != fqn {
		return label
	}
	relabelled := fmt.Sprintf("#%d", inst.ID)
	for _, seg := range rest {
		relabelled += "." + seg.text
	}
	return relabelled
}

// walkObjectPath follows segments through feature values from inst, labelled
// label, to the object they reach. Each segment must name a feature of the
// object before it that holds an object — one, or one picked by index from a
// multi-valued feature — and the first that does not is reported. The label
// spells walked features after `.`, which only ever reads as a feature.
func (s *Session) walkObjectPath(ctx *runtime.Context, inst *runtime.Instance, label string, segments []objectSegment) (*runtime.Instance, string, error) {
	for _, seg := range segments {
		fv, err := inst.GetFeatureValue(ctx, seg.name)
		if err != nil {
			if _, has := inst.FeatureValues[seg.name]; !has {
				return nil, "", pathError(label, seg, "%s has no feature %q%s", label, seg.name, s.featureListHint(inst))
			}
			perr := pathError(label, seg, "%s of %s could not be materialized: %v", lexer.NameText(seg.name), label, err)
			perr.Err = err
			return nil, "", perr
		}
		var val runtime.Value
		next := label + "." + lexer.NameText(seg.name)
		if fv.Values.Kind != runtime.ValInvalid {
			elements := collectionElements(fv.Values)
			switch {
			case len(elements) == 0:
				return nil, "", pathError(label, seg, "%s of %s holds no objects", lexer.NameText(seg.name), label)
			case seg.index == 0:
				return nil, "", pathError(label, seg, "%s of %s holds %d %s: pick one by index, %s[1] to %s[%d]",
					lexer.NameText(seg.name), label, len(elements), plural(len(elements), "object", "objects"), lexer.NameText(seg.name), lexer.NameText(seg.name), len(elements))
			case seg.index > len(elements):
				return nil, "", pathError(label, seg, "%s of %s holds %d %s, so %s names none (indexes run from 1 to %d)",
					lexer.NameText(seg.name), label, len(elements), plural(len(elements), "object", "objects"), seg.text, len(elements))
			}
			val = elements[seg.index-1]
			next = fmt.Sprintf("%s[%d]", next, seg.index)
		} else {
			if seg.index > 0 {
				return nil, "", pathError(label, seg, "%s of %s holds one value and takes no index: write %s, not %s",
					lexer.NameText(seg.name), label, lexer.NameText(seg.name), seg.text)
			}
			val = fv.Value
		}
		id, isObject := val.Object()
		switch {
		case val.Kind == runtime.ValInvalid:
			return nil, "", pathError(label, seg, "%s of %s holds no object", lexer.NameText(seg.name), label)
		case !isObject || ctx.HoldsNoValue(val):
			return nil, "", pathError(label, seg, "%s of %s holds a value (%s), not an object", lexer.NameText(seg.name), label, formatValue(ctx, val))
		}
		child, ok := ctx.Instance(id)
		if !ok {
			return nil, "", pathError(label, seg, "%s of %s holds object #%d, which the session no longer has", lexer.NameText(seg.name), label, id)
		}
		inst, label = child, next
	}
	return inst, label, nil
}

// collectionElements is what a multi-valued feature holds, in order.
func collectionElements(val runtime.Value) []runtime.Value {
	switch val.Kind {
	case runtime.ValSequence:
		if val.Sequence() != nil {
			return val.Sequence().Elements()
		}
	case runtime.ValSet:
		if val.Set() != nil {
			return val.Set().Elements()
		}
	}
	return nil
}

// featureListLimit bounds the features an unknown-feature error lists.
const featureListLimit = 12

// featureListHint names the features an object has, so a misspelt one can be
// corrected without another command: the model's own by name, the ones the
// library declares for it by count.
func (s *Session) featureListHint(inst *runtime.Instance) string {
	if len(inst.FeatureValues) == 0 {
		return " (it has no features)"
	}
	var names []string
	library := 0
	for name, fv := range inst.FeatureValues {
		if s.idx != nil && fv.Feature != nil && s.idx.Library(fv.Feature.Symbol) {
			library++
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	more := ""
	if len(names) > featureListLimit {
		more = fmt.Sprintf(", … (%d in all)", len(names))
		names = names[:featureListLimit]
	}
	switch {
	case len(names) == 0:
		return fmt.Sprintf(" (its %d features are all declared by the library)", library)
	case library > 0:
		more += fmt.Sprintf(", and %d more the library declares", library)
	}
	return fmt.Sprintf(" (its features are %s%s)", strings.Join(names, ", "), more)
}

// unresolvedError reports a name nothing declares, in the wording every surface
// uses for it: the parser's diagnostic and the runtime's sentinel.
func unresolvedError(name string) error {
	return fmt.Errorf("%w: %s", runtime.ErrUnresolvedReference, name)
}

// AmbiguousNameError reports a name that matched more than one declaration. It
// is distinct from a name found nowhere: a command may look elsewhere for the
// latter, but must never answer about one of several candidates.
type AmbiguousNameError struct {
	Name string
	FQNs []string
}

func (e *AmbiguousNameError) Error() string {
	return fmt.Sprintf("symbol %q is ambiguous: %s (use a qualified name)", e.Name, strings.Join(e.FQNs, ", "))
}

// ambiguousError reports a name that matched more than one declaration, listing
// the candidates' fully-qualified names rather than picking one of them.
func ambiguousError(name string, matches []*symbols.Symbol, idx *symbols.Index) error {
	fqns := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, sym := range matches {
		fqn := declarationNotation(sym)
		if !seen[fqn] {
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	sort.Strings(fqns)
	return &AmbiguousNameError{Name: notationName(name), FQNs: fqns}
}

// nameTable maps every simple name the session documents declare outside a body
// to its declarations in scope-tree order, so a lookup costs nothing of the model's size.
type nameTable struct {
	scopes []*symbols.Scope // the trees the table was built from
	byName map[string][]*symbols.Symbol
}

// nameTable returns the table over the session's current documents, rebuilt
// when a submission or reset has replaced a scope tree.
func (s *Session) nameTable() *nameTable {
	scopes := s.docScopes()
	if s.names == nil || !slices.Equal(s.names.scopes, scopes) {
		s.names = buildNameTable(scopes)
	}
	return s.names
}

// buildNameTable tabulates every scope tree in turn, a scope's own members before
// its children's; a body-local scope is skipped with everything nested in it.
func buildNameTable(scopes []*symbols.Scope) *nameTable {
	t := &nameTable{scopes: scopes, byName: make(map[string][]*symbols.Symbol)}
	for _, scope := range scopes {
		t.collect(scope)
	}
	return t
}

func (t *nameTable) collect(scope *symbols.Scope) {
	if scope == nil || scope.BodyLocal() {
		return
	}
	for _, name := range scope.MemberNames() {
		t.byName[name] = append(t.byName[name], symbols.PreferDeclared(scope.LookupLocalAll(name))...)
	}
	for _, child := range scope.Children() {
		t.collect(child)
	}
}

// lookup returns every declaration of name in scope-tree order. The slice is
// the table's own and must not be modified.
func (t *nameTable) lookup(name string) []*symbols.Symbol {
	syms := t.byName[name]
	return syms[:len(syms):len(syms)]
}

// sorted returns every declared name, sorted.
func (t *nameTable) sorted() []string {
	out := make([]string, 0, len(t.byName))
	for name := range t.byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// docScopes returns the scope trees of the session's open documents, in
// buffer order, so a lookup reads both languages.
func (s *Session) docScopes() []*symbols.Scope {
	var out []*symbols.Scope
	for _, doc := range s.sessionDocs() {
		if doc.Scope != nil {
			out = append(out, doc.Scope)
		}
	}
	return out
}

// scopeSymbolForAny is scopeSymbolFor over several scope trees, first hit wins.
func scopeSymbolForAny(scopes []*symbols.Scope, decl ast.Node) *symbols.Symbol {
	for _, scope := range scopes {
		if sym := scopeSymbolFor(scope, decl); sym != nil {
			return sym
		}
	}
	return nil
}

// scopeSymbolFor returns the symbol in scope's tree declared by decl, or nil.
func scopeSymbolFor(scope *symbols.Scope, decl ast.Node) *symbols.Symbol {
	if scope == nil || decl == nil {
		return nil
	}
	for _, sym := range scope.Members() {
		if sym.Decl == decl {
			return sym
		}
	}
	for _, child := range scope.Children() {
		if sym := scopeSymbolFor(child, decl); sym != nil {
			return sym
		}
	}
	return nil
}
