package runtime

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Shapes records the resolved shape of every type a set of objects was
// materialized against, taken while that resolution is still the current one. It
// is what a later context compares its own resolution against to decide whether
// those objects still mean the same thing.
type Shapes struct {
	digests map[string]string // type FQN → resolved shape
	types   []string          // the FQNs above, in the order they were reached
	unnamed bool              // an object of a type with no qualified name
}

// AdoptError says which object could not be carried over into a new context, and
// why, so the loss is reported rather than silently absorbed.
type AdoptError struct {
	Type   string // qualified name of the object's type, as far as it is known
	Reason string
}

func (e *AdoptError) Error() string {
	if e.Type == "" {
		return "cannot carry the object over: " + e.Reason
	}
	return fmt.Sprintf("cannot carry the object of %s over: %s", e.Type, e.Reason)
}

// ShapesOf records the shapes obj and everything it holds were materialized
// against: the objects reachable through its feature values and connector ends, plus the
// variants its values selected. A connector no name reaches is materialized
// again rather than carried, so it is no part of this.
func (ctx *Context) ShapesOf(obj *Instance) *Shapes {
	shapes := &Shapes{digests: make(map[string]string)}
	ctx.recordShapes(obj, shapes, make(map[int64]bool))
	return shapes
}

// ShapesOfType records the shape of one declaration as this context resolves it,
// which is what state held over a declaration rather than an object — a
// debugging session over an action — is invalidated by a change to.
func (ctx *Context) ShapesOfType(sym *symbols.Symbol) *Shapes {
	shapes := &Shapes{digests: make(map[string]string)}
	ctx.recordShape(sym, shapes)
	return shapes
}

// Changed returns the declaration this context no longer resolves the way shapes
// recorded it, so a caller can name what invalidated the state it took those
// shapes for. They were recorded outwards, so reading them back names the one
// that changed rather than one that only holds it.
func (ctx *Context) Changed(shapes *Shapes) (string, bool) {
	if shapes == nil || ctx.model.resolver == nil || ctx.model.resolver.Index() == nil {
		return "", false
	}
	for i := len(shapes.types) - 1; i >= 0; i-- {
		fqn := shapes.types[i]
		if !ctx.resolvesTo(fqn, shapes.digests[fqn]) {
			return fqn, true
		}
	}
	return "", false
}

// resolvesTo reports whether some declaration of the qualified name still has
// the recorded shape.
func (ctx *Context) resolvesTo(fqn, digest string) bool {
	for _, cand := range ctx.model.resolver.Index().LookupQualified(fqn) {
		if ctx.ShapeDigest(cand) == digest {
			return true
		}
	}
	return false
}

func (ctx *Context) recordShapes(obj *Instance, shapes *Shapes, seen map[int64]bool) {
	if obj == nil || seen[obj.ID] {
		return
	}
	seen[obj.ID] = true
	ctx.recordShape(obj.Type, shapes)
	for _, c := range obj.classifiers {
		ctx.recordShape(c, shapes)
	}
	for _, val := range obj.held() {
		ctx.walkValue(val, func(v Value) {
			if v.Kind == ValVariant {
				ctx.recordShape(v.Variant(), shapes)
			}
			if self := v.FunctionSelf(); self != nil {
				ctx.recordShapes(self, shapes, seen)
			}
			if id, ok := carriedObject(v); ok {
				if held, found := ctx.instances[id]; found {
					ctx.recordShapes(held, shapes, seen)
				}
			}
		})
	}
}

// recordShape records the shape of a declaration state was taken against, or
// notes that there is none to compare it by: a declaration of no name of its own
// is reached by no name in a later resolution, whatever scope it sits in.
func (ctx *Context) recordShape(sym *symbols.Symbol, shapes *Shapes) {
	if sym == nil || sym.Name == "" || ctx.fqnOf(sym) == "" {
		shapes.unnamed = true
		return
	}
	ctx.recordReached(sym, shapes)
}

// recordReached records sym's shape and the shapes of the types its features
// hold: a change to any of them is a change to what an object of sym holds.
func (ctx *Context) recordReached(sym *symbols.Symbol, shapes *Shapes) {
	fqn := ctx.fqnOf(sym)
	if fqn == "" {
		return
	}
	if _, done := shapes.digests[fqn]; done {
		return
	}
	shapes.digests[fqn] = ctx.ShapeDigest(sym)
	shapes.types = append(shapes.types, fqn)
	if _, named := ctx.libraryShapeIdentity(sym); named {
		return
	}
	features := ctx.FeaturesOf(sym)
	for i := range features {
		ctx.recordReached(features[i].Type, shapes)
	}
}

// featureNames lists the object's feature names in a fixed order, so what is done
// over its feature values does not depend on map order.
func (obj *Instance) featureNames() []string {
	names := make([]string, 0, len(obj.FeatureValues))
	for name := range obj.FeatureValues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// held returns the values the object carries: its feature values and its connector ends.
func (obj *Instance) held() []Value {
	names := obj.featureNames()
	out := make([]Value, 0, 2*len(names)+len(obj.Ends))
	for _, name := range names {
		fv := obj.FeatureValues[name]
		out = append(out, fv.Value, fv.Values)
	}
	for _, end := range obj.Ends {
		out = append(out, end.Value)
	}
	return out
}

// carried returns the values the object keeps across a carry-over: what a value
// expression states is left out, since it is derived again rather than kept.
func (obj *Instance) carried(ctx *Context) []Value {
	names := obj.featureNames()
	out := make([]Value, 0, 2*len(names)+len(obj.Ends))
	for _, name := range names {
		fv := obj.FeatureValues[name]
		if ctx.derivedFeatureValue(fv) || ctx.connectorFeatureValue(fv) {
			continue
		}
		out = append(out, fv.Value, fv.Values)
	}
	for _, end := range obj.Ends {
		out = append(out, end.Value)
	}
	return out
}

// derivedFeatureValue reports whether the feature value holds what a value expression states,
// which a new context computes again from the declarations that expression now
// reads. What a variation's default states is a variant rather than a value, so
// the object bound to it is carried instead of bound again. A value a run wrote
// is the object's own state, which no default derives again.
func (ctx *Context) derivedFeatureValue(s *FeatureValue) bool {
	if s.Written || s.Feature == nil || !ctx.valueBinds(s.Feature) {
		return false
	}
	return !ctx.model.semantics.IsVariationFeature(s.Feature.Symbol)
}

// collectedFeatureValue reports whether the feature value holds values copied out of the features
// subsetting it, which are read again here since one of them may be derived from
// a declaration that changed. A collection of objects is kept: they are carried.
func (ctx *Context) collectedFeatureValue(s *FeatureValue) bool {
	if s.Written || !s.Materialized || (s.Values.Kind != ValSequence && s.Values.Kind != ValSet) {
		return false
	}
	object := false
	ctx.walkValue(s.Values, func(v Value) {
		if _, held := carriedObject(v); held {
			object = true
		}
	})
	return !object
}

// carriedObject is the object a value denotes or holds (an array's or vector's
// source, a tensor's reference): what a carry-over takes along with the value.
func carriedObject(v Value) (int64, bool) {
	if id, ok := v.Object(); ok {
		return id, true
	}
	id := keptObject(v)
	return id, id != 0
}

// connectorFeatureValue reports whether the feature value holds the object of a connector, whose
// ends a new context attaches again rather than keeping what they read before.
func (ctx *Context) connectorFeatureValue(s *FeatureValue) bool {
	return s.Feature != nil && ctx.model.semantics.IsConnectorUsage(s.Feature.Symbol)
}

// HoldsObject reports whether the value is, or carries, an object of this context:
// one a reader can inspect, so the value is only meaningful in the context it came from.
func (ctx *Context) HoldsObject(val Value) bool {
	found := false
	ctx.walkValue(val, func(v Value) {
		if _, ok := carriedObject(v); ok {
			found = true
		}
	})
	return found
}

// walkValue visits a value and everything nested in it: the elements of a
// collection, the frame a vector is over, the frames and placement values a
// transformation relates.
func (ctx *Context) walkValue(val Value, visit func(Value)) {
	ctx.walkValueFrom(val, visit, make(map[*CoordinateFrame]bool))
}

// walkValueFrom is walkValue past the frames already visited, which a frame's own
// transformation names again as its target.
func (ctx *Context) walkValueFrom(val Value, visit func(Value), seen map[*CoordinateFrame]bool) {
	visit(val)
	switch val.Kind {
	case ValVectorQuantity:
		if frame := val.VectorQuantity().Frame; frame != nil && !seen[frame] {
			ctx.walkValueFrom(NewCoordinateFrameValue(frame), visit, seen)
		}
	case ValCoordinateFrame:
		frame := val.CoordinateFrame()
		if seen[frame] {
			return
		}
		seen[frame] = true
		if frame.Transformation != nil {
			ctx.walkValueFrom(NewCoordinateTransformationValue(frame.Transformation), visit, seen)
		}
	case ValCoordinateTransformation:
		t := val.CoordinateTransformation()
		for _, frame := range []*CoordinateFrame{t.Source, t.Target} {
			if frame != nil && !seen[frame] {
				ctx.walkValueFrom(NewCoordinateFrameValue(frame), visit, seen)
			}
		}
		for _, held := range t.placementValues() {
			ctx.walkValueFrom(held, visit, seen)
		}
	case ValSequence:
		if val.Sequence() != nil {
			for _, elem := range val.Sequence().Elements() {
				ctx.walkValueFrom(elem, visit, seen)
			}
		}
	case ValSet:
		if val.Set() != nil {
			for _, elem := range val.Set().Elements() {
				ctx.walkValueFrom(elem, visit, seen)
			}
		}
	case ValArray:
		for _, elem := range val.Array().Elements {
			ctx.walkValueFrom(elem, visit, seen)
		}
	}
}

// ShapeDigest renders what instantiating a type produces as this context
// resolves it now: the features an object of it gets, with the type,
// multiplicity and default of each, and the shapes of the types those features
// hold. Two contexts that agree on the digest agree on the object.
func (ctx *Context) ShapeDigest(sym *symbols.Symbol) string {
	var b strings.Builder
	ctx.writeShape(&b, sym, make(map[string]bool))
	return b.String()
}

func (ctx *Context) writeShape(b *strings.Builder, sym *symbols.Symbol, open map[string]bool) {
	if sym == nil {
		b.WriteString("<untyped>")
		return
	}
	fqn := ctx.fqnOf(sym)
	if fqn == "" {
		fqn = "<unnamed>"
	}
	fmt.Fprintf(b, "%s/%s", fqn, sym.Kind)
	// Over the same library a type resolves the same way in every context, so its
	// name and the library's identity say all its expansion would.
	if identity, named := ctx.libraryShapeIdentity(sym); named {
		fmt.Fprintf(b, "#%s", identity)
		return
	}
	// A type reached through its own features is named rather than expanded
	// again, so a recursive shape has a finite digest.
	if open[fqn] {
		b.WriteString("…")
		return
	}
	open[fqn] = true
	defer delete(open, fqn)
	b.WriteString("{")
	features := ctx.FeaturesOf(sym)
	for i := range features {
		feat := &features[i]
		fmt.Fprintf(b, "%s:%s..%s", feat.Name, bound(feat.Multiplicity.Lower), bound(feat.Multiplicity.Upper))
		if ctx.model.semantics.IsVariationFeature(feat.Symbol) {
			b.WriteString("|variation")
		}
		if feat.DefaultValue != nil {
			owner := feat.DefaultDecl
			if owner == nil {
				owner = feat.Symbol
			}
			fmt.Fprintf(b, "=%s", ctx.declText(owner, feat.DefaultValue.Span()))
		}
		// A body governing over an inherited value is what materializing reads,
		// so the shape follows edits confined to that body.
		if feat.DefaultValue != nil && ctx.bodyGovernsInheritedValue(feat) && feat.Symbol.Decl != nil {
			fmt.Fprintf(b, "|body:%s", ctx.declText(feat.Symbol, feat.Symbol.Decl.Span()))
		}
		b.WriteString("@")
		ctx.writeShape(b, feat.Type, open)
		b.WriteString(";")
	}
	ctx.writeBoundBehaviors(b, sym)
	b.WriteString("}")
}

// writeBoundBehaviors renders the behaviors a type binds to its objects as the
// bodies they now state: an object of a type whose machine was rewritten runs a
// behavior the one carried over does not, so it is not the same object.
func (ctx *Context) writeBoundBehaviors(b *strings.Builder, sym *symbols.Symbol) {
	for _, decl := range ctx.classifierBehaviorsOf(sym) {
		fmt.Fprintf(b, "!%s %s=%s;", decl.behavior.Kind, decl.behavior.Name, ctx.behaviorText(decl))
	}
}

// behaviorText renders the body a binding declaration runs as written, or says
// that it names none.
func (ctx *Context) behaviorText(decl classifierBehaviorDecl) string {
	body, err := ctx.classifierBehaviorSymbol(decl)
	if err != nil || body == nil || body.Decl == nil {
		return "<unresolved>"
	}
	return ctx.declText(body, body.Decl.Span())
}

func bound(b semantics.Bound) string {
	switch {
	case !b.Known:
		return "?"
	case b.Infinite:
		return "*"
	default:
		return fmt.Sprint(b.Value)
	}
}

// declText renders the text a declaration wrote at the given span, falling back
// to the span itself for a file whose text this context was not given — an
// unchanged library file, where the span is as stable as the text.
func (ctx *Context) declText(owner *symbols.Symbol, span source.Span) string {
	file := ""
	if owner != nil {
		file = owner.DocName
	}
	return ctx.textIn(file, span)
}

// textIn renders the text the named document wrote at the given span, falling
// back to the span for a document whose text this context was not given.
func (ctx *Context) textIn(file string, span source.Span) string {
	if sf, ok := ctx.model.sources[file]; ok && span.End() <= sf.Len() {
		return strings.Join(strings.Fields(sf.Text(span)), " ")
	}
	return fmt.Sprintf("%s#%d+%d", file, span.Offset, span.Len)
}

// libraryShapeIdentity is the digest of the library declaring sym, when the index
// states one; a library of unknown text is expanded like the model.
func (ctx *Context) libraryShapeIdentity(sym *symbols.Symbol) (string, bool) {
	if !ctx.libraryTier(sym).Library() {
		return "", false
	}
	return ctx.model.resolver.Index().LibraryIdentity()
}

func (ctx *Context) fqnOf(sym *symbols.Symbol) string {
	if sym == nil || ctx.model.resolver == nil {
		return ""
	}
	idx := ctx.model.resolver.Index()
	if idx == nil {
		return ""
	}
	return idx.GetFQN(sym)
}

// Adopt takes obj — and every object it holds — over from prev into this
// context, keeping the identity and the values they carry across a re-analysis
// of the document they were materialized from. Each object is rebound to the
// declaration of the same qualified name here, which must resolve to the shape
// recorded in shapes; anything else is refused with the context left untouched.
// The objects are moved rather than copied, so prev holds them too afterwards
// and this context takes over its identity sequence; nothing new should be
// materialized through prev, which registers it there alone.
//
// A behavior a carried object ran belongs to the analysis it started in, so it is
// started again here from its initial state rather than continued. The behaviors
// restarted are returned, so the carry-over reports what it cost.
func (ctx *Context) Adopt(prev *Context, shapes *Shapes, obj *Instance) ([]string, error) {
	if prev == nil || shapes == nil || obj == nil {
		return nil, &AdoptError{Reason: "there is nothing to carry over"}
	}
	if shapes.unnamed {
		return nil, &AdoptError{Reason: "it is of a declaration with no qualified name"}
	}
	a := &adoption{ctx: ctx, prev: prev, shapes: shapes,
		plans:   make(map[int64]*adoptPlan),
		rebound: make(map[*symbols.Symbol]*symbols.Symbol),
	}
	if err := a.plan(obj); err != nil {
		return nil, err
	}
	a.commit()
	return a.restartBehaviors()
}

// adoption is one carry-over: what it has planned, and the symbols it rebound
// while planning, applied only once the whole closure has been accepted.
type adoption struct {
	ctx     *Context
	prev    *Context
	shapes  *Shapes
	plans   map[int64]*adoptPlan
	rebound map[*symbols.Symbol]*symbols.Symbol
	mark    int // how many objects ctx had registered before the carry-over
}

// adoptPlan is what one object becomes in the new context: the declaration it is
// of, the features that classified it, and the feature each of its feature
// values fills.
type adoptPlan struct {
	obj         *Instance
	typeSym     *symbols.Symbol
	classifiers []*symbols.Symbol
	features    map[string]*EffectiveFeature
}

// featureFor returns the feature a feature value reached under name fills: the one its
// own name gives, which for a feature value shared by several names of one feature is the
// name it was created under.
func (p *adoptPlan) featureFor(name string, fv *FeatureValue) *EffectiveFeature {
	if fv.Feature != nil {
		if feat, ok := p.features[fv.Feature.Name]; ok {
			return feat
		}
	}
	return p.features[name]
}

func (a *adoption) plan(obj *Instance) error {
	if obj == nil {
		return &AdoptError{Reason: "there is nothing to carry over"}
	}
	if _, planned := a.plans[obj.ID]; planned {
		return nil
	}
	// An object already in this context was carried over with another root; a
	// different object holding its ID is a collision that must not be papered
	// over by overwriting either of them.
	if existing, ok := a.ctx.instances[obj.ID]; ok {
		if existing == obj {
			return nil
		}
		return &AdoptError{Type: a.ctx.fqnOf(obj.Type), Reason: fmt.Sprintf("its identity %d is taken", obj.ID)}
	}
	typeSym, err := a.rebindShaped(obj.Type, "its type")
	if err != nil {
		return err
	}
	fqn := a.ctx.fqnOf(typeSym)
	declared := &declaredFeatures{byName: make(map[string]*EffectiveFeature), bySymbol: make(map[*symbols.Symbol]*EffectiveFeature)}
	a.indexFeatures(typeSym, declared)
	// A feature that classified the object declares features of it too, so the
	// object is rebound to what that feature is declared as here.
	classifiers := make([]*symbols.Symbol, 0, len(obj.classifiers))
	for _, c := range obj.classifiers {
		found, err := a.rebindShaped(c, "a feature that held it")
		if err != nil {
			return err
		}
		classifiers = append(classifiers, found)
		a.indexFeatures(found, declared)
	}
	plan := &adoptPlan{obj: obj, typeSym: typeSym, classifiers: classifiers,
		features: make(map[string]*EffectiveFeature, len(obj.FeatureValues))}
	for _, name := range obj.featureNames() {
		feat, err := a.planFeature(fqn, typeSym, name, obj.FeatureValues[name], declared)
		if err != nil {
			return err
		}
		plan.features[name] = feat
	}
	a.plans[obj.ID] = plan
	for _, val := range obj.carried(a.prev) {
		if err := a.planValue(fqn, val); err != nil {
			return err
		}
	}
	return nil
}

// rebindShaped rebinds a declaration an object was materialized against and
// checks that it still resolves to the shape recorded for it.
func (a *adoption) rebindShaped(sym *symbols.Symbol, what string) (*symbols.Symbol, error) {
	found, err := a.rebind(sym, what)
	if err != nil {
		return nil, err
	}
	fqn := a.ctx.fqnOf(found)
	want, recorded := a.shapes.digests[fqn]
	if !recorded {
		return nil, &AdoptError{Type: fqn, Reason: "the shape it was materialized against was not recorded"}
	}
	if a.ctx.ShapeDigest(found) != want {
		return nil, &AdoptError{Type: fqn, Reason: "its declaration resolves to a different shape now"}
	}
	return found, nil
}

// declaredFeatures indexes what an object's types declare for it here: by the
// declaration a feature reads, and by name for the first type to give the name.
type declaredFeatures struct {
	byName   map[string]*EffectiveFeature
	bySymbol map[*symbols.Symbol]*EffectiveFeature
}

// indexFeatures adds the features sym declares to declared, keeping the names a
// declaration indexed before it already gave.
func (a *adoption) indexFeatures(sym *symbols.Symbol, declared *declaredFeatures) {
	features := a.ctx.FeaturesOf(sym)
	for i := range features {
		feat := &features[i]
		if _, taken := declared.byName[feat.Name]; !taken {
			declared.byName[feat.Name] = feat
		}
		if _, taken := declared.bySymbol[feat.Symbol]; feat.Symbol != nil && !taken {
			declared.bySymbol[feat.Symbol] = feat
		}
	}
}

// planFeature is the feature a feature value fills in this context: the declaration it
// read, rebound (a classifier's refinement of a carried feature stays the one read), else
// the one its name gives, or — for a feature value a connector added, which no
// declaration of the type carries — the recorded one with its symbols rebound.
func (a *adoption) planFeature(owner string, typeSym *symbols.Symbol, name string, fv *FeatureValue, declared *declaredFeatures) (*EffectiveFeature, error) {
	if feat := a.declarationRead(fv, declared); feat != nil {
		return feat, nil
	}
	if feat, ok := declared.byName[name]; ok {
		return feat, nil
	}
	if fv.Feature == nil || fv.Feature.DefaultValue != nil {
		return nil, &AdoptError{Type: owner, Reason: fmt.Sprintf("it no longer has a feature %q", name)}
	}
	feat := *fv.Feature
	feat.OwnerType = typeSym
	for _, ref := range []**symbols.Symbol{&feat.Symbol, &feat.Type} {
		if *ref == nil {
			continue
		}
		found, err := a.rebind(*ref, fmt.Sprintf("the feature %q", name))
		if err != nil {
			return nil, err
		}
		*ref = found
	}
	return &feat, nil
}

// declarationRead is the feature declared here by the rebound declaration fv reads,
// when one of the object's types declares it.
func (a *adoption) declarationRead(fv *FeatureValue, declared *declaredFeatures) *EffectiveFeature {
	if fv.Feature == nil || fv.Feature.Symbol == nil {
		return nil
	}
	read, err := a.rebind(fv.Feature.Symbol, fmt.Sprintf("the feature %q", fv.Feature.Name))
	if err != nil {
		return nil
	}
	return declared.bySymbol[read]
}

func (a *adoption) planValue(owner string, val Value) error {
	var err error
	a.prev.walkValue(val, func(v Value) {
		if err != nil {
			return
		}
		// An unevaluated expression reads names in the document that just
		// changed, so it cannot be carried over as the value it stands for.
		if v.Kind == ValExpr {
			err = &AdoptError{Type: owner, Reason: "it holds an expression that was never evaluated"}
			return
		}
		if v.Kind == ValUndetermined {
			err = &AdoptError{Type: owner, Reason: "it holds a value the model does not determine"}
			return
		}
		// A function value denotes its calc by name, so it is rebound as a variant is;
		// the object it closes over is carried with it.
		if v.Kind == ValFunction {
			if err = a.planFunction(owner, v); err != nil {
				return
			}
		}
		if v.Kind == ValVariant {
			if _, rebindErr := a.rebind(v.Variant(), "a variant it selected"); rebindErr != nil {
				err = rebindErr
				return
			}
		}
		if v.Kind == ValMetaobject {
			if err = a.planMetaobject(v); err != nil {
				return
			}
		}
		for _, unit := range unitsOf(v) {
			if err = a.planUnit(unit); err != nil {
				return
			}
		}
		switch v.Kind {
		case ValCoordinateFrame:
			err = a.planFrame(v.CoordinateFrame())
		case ValCoordinateTransformation:
			err = a.planTransformation(v.CoordinateTransformation())
		}
		if err != nil {
			return
		}
		if id, ok := carriedObject(v); ok {
			err = a.planHeld(owner, id)
		}
	})
	return err
}

// planFunction rebinds the calc a function value is of to its declaration here,
// refusing one that is no longer a calc that can be invoked.
func (a *adoption) planFunction(owner string, v Value) error {
	held := "the function " + v.FunctionName() + " it holds"
	if v.FunctionClosesOverBody() {
		return &AdoptError{Type: owner, Reason: held + " closes over the bindings of a run that has ended"}
	}
	found, err := a.rebind(v.Function(), held)
	if err != nil {
		return err
	}
	if _, err := a.ctx.calcShapeOf(found); err != nil {
		return &AdoptError{Type: owner, Reason: held + " cannot be invoked here: " + err.Error()}
	}
	if self := v.FunctionSelf(); self != nil {
		return a.planHeld(owner, self.ID)
	}
	return nil
}

// unitsOf is the measurement units a quantity, reference or empty quantity
// sequence names.
func unitsOf(v Value) []Unit {
	switch v.Kind {
	case ValQuantity:
		return []Unit{v.Quantity().Unit}
	case ValMeasurementRef:
		return []Unit{v.MeasurementRef().Unit}
	case ValVectorQuantity:
		return v.VectorQuantity().Units
	case ValTensorQuantity:
		return v.TensorQuantity().Units
	case ValSequence:
		if unit, ok := v.Sequence().ElementUnit(); ok {
			return []Unit{unit}
		}
	}
	return nil
}

// planUnit rebinds every unit declaration a unit product names, and refuses a
// unit whose reduction the re-analysis changed (its magnitude would read wrong).
func (a *adoption) planUnit(unit Unit) error {
	if unit.Product.IsEmpty() {
		return nil
	}
	term := semantics.UnitTerm{Scale: semantics.UnitScale(1)}
	for _, power := range unit.Product.Powers {
		what := "the unit " + power.Name + " it is measured in"
		var reduces semantics.UnitTerm
		switch {
		case power.Unit != nil:
			found, err := a.rebind(power.Unit, what)
			if err != nil {
				return err
			}
			if reduces, err = a.ctx.model.semantics.UnitTermOf(found); err != nil {
				return &AdoptError{Type: a.ctx.fqnOf(found), Reason: what + " no longer reduces: " + err.Error()}
			}
		case power.Reduces != nil:
			if err := a.planTerm(*power.Reduces); err != nil {
				return err
			}
			reduces = a.rewriteTerm(*power.Reduces)
		default:
			return nil
		}
		term = term.Times(reduces.Pow(power.Exponent))
	}
	if err := a.planTerm(unit.Term); err != nil {
		return err
	}
	if was := a.rewriteTerm(unit.Term); !term.Same(was) {
		return &AdoptError{Reason: "the unit " + unit.Product.String() + " it is measured in now reduces to " +
			term.String() + ", not " + was.String()}
	}
	return nil
}

// planTerm rebinds every base unit a reduction is expressed over.
func (a *adoption) planTerm(term semantics.UnitTerm) error {
	for _, factor := range term.Factors {
		if factor.Unit == nil {
			continue
		}
		if _, err := a.rebind(factor.Unit, "the base unit "+factor.Unit.Name+" it reduces to"); err != nil {
			return err
		}
	}
	return nil
}

// rewriteUnit is the unit with every declaration it names rebound, in its
// product and in its reduction alike.
func (a *adoption) rewriteUnit(unit Unit) Unit {
	powers := make([]semantics.UnitPower, len(unit.Product.Powers))
	for i, power := range unit.Product.Powers {
		if found, ok := a.rebound[power.Unit]; ok {
			power.Unit = found
		}
		if power.Reduces != nil {
			reduces := a.rewriteTerm(*power.Reduces)
			power.Reduces = &reduces
		}
		powers[i] = power
	}
	unit.Product = semantics.UnitProduct{Powers: powers}
	unit.Term = a.rewriteTerm(unit.Term)
	return unit
}

// rewriteTerm is the reduction with every base unit it names rebound.
func (a *adoption) rewriteTerm(term semantics.UnitTerm) semantics.UnitTerm {
	factors := make([]semantics.UnitFactor, len(term.Factors))
	for i, factor := range term.Factors {
		if found, ok := a.rebound[factor.Unit]; ok {
			factor.Unit = found
		}
		factors[i] = factor
	}
	term.Factors = factors
	return term
}

func (a *adoption) planHeld(owner string, id int64) error {
	held, ok := a.prev.instances[id]
	if !ok {
		return &AdoptError{Type: owner, Reason: fmt.Sprintf("the object %d it holds is gone", id)}
	}
	return a.plan(held)
}

// rebind maps a symbol of the previous context to the one declaration of the
// same qualified name and kind here — as the scope tree the caller resolves in
// declares it — and records it for the values that name it.
func (a *adoption) rebind(sym *symbols.Symbol, what string) (*symbols.Symbol, error) {
	if sym == nil {
		return nil, &AdoptError{Reason: what + " was never resolved"}
	}
	if found, ok := a.rebound[sym]; ok {
		return found, nil
	}
	fqn := a.prev.fqnOf(sym)
	if fqn == "" {
		fqn = a.ctx.fqnOf(sym)
	}
	if fqn == "" {
		return nil, &AdoptError{Reason: what + " has no qualified name"}
	}
	idx := a.ctx.model.resolver.Index()
	var found *symbols.Symbol
	for _, cand := range idx.LookupQualified(fqn) {
		if cand.Kind != sym.Kind {
			continue
		}
		if found != nil {
			return nil, &AdoptError{Type: fqn, Reason: what + " is now declared more than once"}
		}
		found = cand
	}
	if found == nil {
		return nil, &AdoptError{Type: fqn, Reason: what + " is no longer declared"}
	}
	found = a.ctx.declaredSymbol(found)
	a.rebound[sym] = found
	return found, nil
}

// AdoptIdentities takes over the identity sequence of a context this one
// replaces, without carrying any object over: objects a run started before still
// materializes through it, so neither context may hand out the other's.
func (ctx *Context) AdoptIdentities(prev *Context) {
	if prev == nil || prev == ctx || prev.ids == nil || prev.ids == ctx.ids {
		return
	}
	prev.ids.share(ctx)
}

// commit moves the planned objects into this context, rebinding what each of
// them points at and taking over the derived state that is about them.
func (a *adoption) commit() {
	a.mark = len(a.ctx.created)
	a.ctx.AdoptIdentities(a.prev)
	adopted := make(map[int64]bool, len(a.plans))
	for id, plan := range a.plans {
		adopted[id] = true
		prevTypes := plan.obj.types()
		plan.obj.Type = plan.typeSym
		plan.obj.classifiers = plan.classifiers
		// Names of one redefined feature share a feature value, which is rebound once, to
		// the feature of the name the shared feature value was created under.
		done := make(map[*FeatureValue]bool, len(plan.obj.FeatureValues))
		for _, name := range plan.obj.featureNames() {
			fv := plan.obj.FeatureValues[name]
			if done[fv] {
				continue
			}
			done[fv] = true
			fv.Feature = plan.featureFor(name, fv)
			// What fv read, and what read it, did so in the previous analysis: no edge
			// is kept between the two, and a value derived again here lists itself anew.
			fv.dependents = nil
			a.ctx.forgetReads(fv)
			// A value an expression states is derived again here, so it cannot go
			// stale against what that expression now reads.
			if a.ctx.derivedFeatureValue(fv) {
				fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
				continue
			}
			if a.ctx.collectedFeatureValue(fv) {
				fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
				continue
			}
			// A connector reads the features the `connect` clause names, which are
			// read again here — under the identity its object had, which names the
			// same connector.
			if a.ctx.connectorFeatureValue(fv) {
				if id, held := fv.Value.Object(); held {
					plan.obj.keepConnector(fv, id)
				}
				fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
				continue
			}
			fv.Value = a.rewrite(fv.Value)
			fv.Values = a.rewrite(fv.Values)
		}
		for i := range plan.obj.Ends {
			plan.obj.Ends[i].Value = a.rewrite(plan.obj.Ends[i].Value)
		}
		// The connectors the owner names no name are reached by no name here, so
		// they are materialized again against the declarations as they are now —
		// under the identities they had, which name the same connectors.
		plan.obj.keepAnonymous(a.ctx, a.prev, prevTypes)
		a.ctx.registerInstance(plan.obj)
		a.ctx.claimID(id)
	}
	a.beginLives()
	a.carryDerived(adopted)
}

// beginLives gives the carried objects their lifetimes here, in identity order
// and each whole before its parts, so a part begins with its whole.
func (a *adoption) beginLives() {
	for _, id := range a.plannedIDs() {
		a.beginLifeOf(a.plans[id].obj)
	}
}

// beginLifeOf begins obj's lifetime after the carried whole that holds it.
func (a *adoption) beginLifeOf(obj *Instance) {
	if _, begun := a.ctx.lives[obj.ID]; begun {
		return
	}
	if owner := obj.owner; owner != nil {
		if plan, carried := a.plans[owner.ID]; carried && plan.obj == owner {
			a.beginLifeOf(owner)
		}
	}
	a.ctx.beginLife(obj)
	a.ctx.carryLife(a.prev, obj)
}

// restartBehaviors runs the carried objects' behaviors again in this context, from
// their initial states, since an execution holds the graph, symbols and message bus
// of the analysis it started in. A start that fails takes the objects with it.
func (a *adoption) restartBehaviors() ([]string, error) {
	var objects []*Instance
	var restarted []string
	carried := make([]*Instance, 0, len(a.plans))
	for _, id := range a.plannedIDs() {
		obj := a.plans[id].obj
		carried = append(carried, obj)
		if len(obj.behaviors) == 0 {
			continue
		}
		// A destroyed object performs nothing any more: its behaviors are dropped, not restarted.
		if a.ctx.lives[obj.ID].destroyed {
			obj.behaviors = nil
			continue
		}
		for _, behavior := range obj.behaviors {
			restarted = append(restarted, behavior.Describe())
		}
		obj.behaviors = nil
		objects = append(objects, obj)
	}
	if len(objects) == 0 {
		return nil, nil
	}
	// A behavior writes the feature values of its own object and of the objects that
	// one holds, so the whole carried closure forgets what the discarded run wrote.
	for _, obj := range carried {
		obj.forgetBehaviorWrites(a.ctx)
	}
	if err := a.ctx.restartClassifierBehaviors(objects); err != nil {
		a.abandon()
		return nil, &AdoptError{Type: a.ctx.fqnOf(objects[0].Type),
			Reason: "the behavior it runs could not be started again: " + err.Error()}
	}
	return restarted, nil
}

// plannedIDs lists the carried identities in order, so what is done over them
// does not depend on map order.
func (a *adoption) plannedIDs() []int64 {
	ids := make([]int64, 0, len(a.plans))
	for id := range a.plans {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// abandon takes the carried objects back out of this context, for a carry-over
// that cannot be completed after the objects were moved: them, their lifetimes,
// and whatever the failed restart of their behaviors registered.
func (a *adoption) abandon() {
	a.ctx.abandonInstancesSince(a.mark)
}

// carryDerived takes over the state the previous context derived about the
// objects carried over: which usage denotes which occurrence, which annotation
// denotes which object, and which variant each of them selected. What a rebound
// declaration no longer has is dropped, so it is derived again rather than kept
// wrong.
func (a *adoption) carryDerived(adopted map[int64]bool) {
	for sym, ids := range a.prev.occurrences {
		if slices.ContainsFunc(ids, func(id int64) bool { return !adopted[id] }) {
			continue
		}
		if found, ok := a.rebound[sym]; ok {
			a.ctx.occurrences[found] = ids
			continue
		}
		if found, err := a.rebind(sym, "a usage of it"); err == nil {
			a.ctx.occurrences[found] = ids
		}
	}
	for key, id := range a.prev.metadataObjects {
		if !adopted[id] {
			continue
		}
		element, err := a.rebind(key.element, "the element it annotates")
		if err != nil {
			continue
		}
		// An annotation denotes the object made for it only while it still reads
		// as it read: an edited or reordered one is read again rather than reused.
		stated := a.prev.metadataAnnotationDigest(key.element, key.index)
		if stated == "" || stated != a.ctx.metadataAnnotationDigest(element, key.index) {
			continue
		}
		a.ctx.metadataObjects[metadataAnnotation{element: element, index: key.index}] = id
	}
	for key, id := range a.prev.variantObjects {
		if !adopted[key.owner] || !adopted[id] {
			continue
		}
		variation, err := a.rebind(key.variation, "a variation of it")
		if err != nil {
			continue
		}
		variant, err := a.rebind(key.variant, "a variant of it")
		if err != nil {
			continue
		}
		a.ctx.variantObjects[variantObject{owner: key.owner, variation: variation, variant: variant}] = id
	}
	for key, variant := range a.prev.selectedVariants {
		if adopted[key.owner] {
			a.ctx.selectedVariants[key] = variant
		}
	}
	carried := make(map[*symbols.Symbol]bool)
	for sym := range a.prev.namespaceBindings {
		a.carryBinding(sym, adopted, carried)
	}
}

// carryBinding carries a usage's binding only while everything its value read still reads as
// it did — declarations, names, type hierarchies — bound dependencies first; else it is read again.
func (a *adoption) carryBinding(sym *symbols.Symbol, adopted map[int64]bool, carried map[*symbols.Symbol]bool) bool {
	if done, ok := carried[sym]; ok {
		return done
	}
	carried[sym] = false
	val := a.prev.namespaceBindings[sym]
	found, err := a.rebind(sym, "a usage of it")
	if err != nil || !a.allAdopted(val, adopted) {
		return false
	}
	stated := a.prev.declarationDigest(sym)
	if stated == "" || stated != a.ctx.declarationDigest(found) {
		return false
	}
	reads := a.prev.bindingReads[sym]
	rewritten := newBindingReads()
	if reads != nil {
		if reads.opaque {
			return false
		}
		for dep, digest := range reads.decls {
			depFound, err := a.rebind(dep, "a declaration it read")
			if err != nil || digest != a.ctx.declarationDigest(depFound) {
				return false
			}
			if namespaceObjectUsage(dep) {
				if _, bound := a.prev.namespaceBindings[dep]; !bound || !a.carryBinding(dep, adopted, carried) {
					return false
				}
			}
			if ids, occurs := a.prev.occurrences[dep]; occurs && !slices.Equal(a.ctx.occurrences[depFound], ids) {
				return false
			}
			rewritten.decls[depFound] = digest
		}
		for doc, digest := range reads.docs {
			if digest != a.ctx.documentDigest(doc) {
				return false
			}
			rewritten.docs[doc] = digest
		}
		for typ, digest := range reads.types {
			typFound, err := a.rebind(typ, "a type it judged")
			if err != nil || digest != a.ctx.typeDigest(typFound) {
				return false
			}
			rewritten.types[typFound] = digest
		}
		for read, denoted := range reads.names {
			read, ok := a.rebindNameRead(read)
			if !ok {
				return false
			}
			if now, ok := a.ctx.replay(read); !ok || now != denoted {
				return false
			}
			rewritten.names[read] = denoted
		}
	}
	a.ctx.namespaceBindings[found] = a.rewrite(val)
	a.ctx.bindingReads[found] = rewritten
	carried[sym] = true
	return true
}

// allAdopted reports whether every object a value carries — an element of a collection, the one
// an array, vector, frame or transformation was read from, a function's self — is here: carried
// over now, or by an earlier carry-over from the same previous context.
func (a *adoption) allAdopted(val Value, adopted map[int64]bool) bool {
	all := true
	a.prev.walkValue(val, func(v Value) {
		if id, ok := carriedObject(v); ok && !adopted[id] && !a.carriedEarlier(id) {
			all = false
		}
		if self := v.FunctionSelf(); self != nil && !adopted[self.ID] && !a.carriedEarlier(self.ID) {
			all = false
		}
	})
	return all
}

// carriedEarlier reports whether this context already holds the previous context's object id —
// the very object, not another one that took the identity.
func (a *adoption) carriedEarlier(id int64) bool {
	was, ok := a.prev.instances[id]
	if !ok {
		return false
	}
	now, ok := a.ctx.instances[id]
	return ok && now == was
}

// rebindNameRead names a lookup's scope and hidden declaration in this context.
func (a *adoption) rebindNameRead(read nameRead) (nameRead, bool) {
	if read.scope.owner != nil {
		owner, err := a.rebind(read.scope.owner, "a namespace it looked a name up in")
		if err != nil {
			return read, false
		}
		read.scope.owner = owner
	}
	if read.excluding != nil {
		excluding, err := a.rebind(read.excluding, "a declaration it looked a name up around")
		if err != nil {
			return read, false
		}
		read.excluding = excluding
	}
	return read, true
}

// declarationDigest is the text of a symbol's declaration, as its document states it, and
// what this context resolves it to.
func (ctx *Context) declarationDigest(sym *symbols.Symbol) string {
	if sym == nil || sym.DocName == "" {
		return ""
	}
	return ctx.textIn(sym.DocName, sym.DeclSpan) + "\n" + ctx.typeDigest(sym)
}

// documentDigest is the whole text of a document, as this context was given it.
func (ctx *Context) documentDigest(doc string) string {
	sf, ok := ctx.model.sources[doc]
	if !ok {
		return ""
	}
	return ctx.textIn(doc, source.Span{Len: sf.Len()})
}

// planMetaobject rebinds the element a metaobject denotes and the metaclass it
// is an instance of, both named by qualified name.
func (a *adoption) planMetaobject(v Value) error {
	if _, err := a.rebind(v.MetaobjectElement(), "an element a metaobject denotes"); err != nil {
		return err
	}
	_, err := a.rebind(v.MetaobjectClass(), "the metaclass of a metaobject")
	return err
}

// rewrite returns the value as this context holds it: the same value with every
// symbol it names rebound. Collections are rebuilt rather than edited, since a
// set is keyed on the values it holds.
func (a *adoption) rewrite(val Value) Value {
	switch val.Kind {
	case ValVariant:
		if found, ok := a.rebound[val.Variant()]; ok {
			return NewVariantValue(found, val.Instance)
		}
		return val
	case ValMetaobject:
		element, elementOK := a.rebound[val.MetaobjectElement()]
		metaclass, metaclassOK := a.rebound[val.MetaobjectClass()]
		if !elementOK || !metaclassOK {
			return val
		}
		return NewMetaobject(element, metaclass)
	case ValFunction:
		found, ok := a.rebound[val.Function()]
		if !ok {
			return val
		}
		shape, err := a.ctx.calcShapeOf(found)
		if err != nil {
			return val
		}
		fn := &functionValue{shape: shape, scope: found.OwnerScope, self: val.FunctionSelf()}
		fn.library, _ = a.ctx.libraryFunctionFor(found)
		return Value{Kind: ValFunction, ref: fn}
	case ValSequence:
		if val.Sequence() == nil {
			return val
		}
		if unit, ok := val.Sequence().ElementUnit(); ok {
			return NewEmptySequenceOf(a.rewriteUnit(unit))
		}
		seq := NewSequence()
		for _, elem := range val.Sequence().Elements() {
			seq.Append(a.rewrite(elem))
		}
		return NewSequenceValue(seq)
	case ValSet:
		if val.Set() == nil {
			return val
		}
		set := NewSetIn(a.ctx)
		for _, elem := range val.Set().Elements() {
			set.Add(a.rewrite(elem))
		}
		return NewSetValue(set)
	case ValArray:
		arr := val.Array()
		elements := make([]Value, len(arr.Elements))
		for i, elem := range arr.Elements {
			elements[i] = a.rewrite(elem)
		}
		out := NewArrayValue(arr.Dimensions, elements)
		out.Array().Object = arr.Object
		return out
	case ValQuantity:
		q := *val.Quantity()
		q.Unit = a.rewriteUnit(q.Unit)
		return NewQuantityValue(&q)
	case ValMeasurementRef:
		return NewMeasurementRefValue(a.rewriteUnit(val.MeasurementRef().Unit))
	case ValVectorQuantity:
		vq := val.VectorQuantity()
		units := make([]Unit, len(vq.Units))
		for i, unit := range vq.Units {
			units[i] = a.rewriteUnit(unit)
		}
		if vq.Frame != nil {
			return NewFramedVectorQuantityValue(vq.Num, a.rewriteFrame(vq.Frame, map[*CoordinateFrame]*CoordinateFrame{}))
		}
		return NewVectorQuantityValue(vq.Num, units)
	case ValTensorQuantity:
		tq := *val.TensorQuantity()
		tq.Units = make([]Unit, len(tq.Units))
		for i, unit := range val.TensorQuantity().Units {
			tq.Units[i] = a.rewriteUnit(unit)
		}
		return Value{Kind: ValTensorQuantity, ref: &tq}
	case ValCoordinateFrame:
		return NewCoordinateFrameValue(a.rewriteFrame(val.CoordinateFrame(), map[*CoordinateFrame]*CoordinateFrame{}))
	case ValCoordinateTransformation:
		return NewCoordinateTransformationValue(a.rewriteTransformation(val.CoordinateTransformation(), map[*CoordinateFrame]*CoordinateFrame{}))
	default:
		return val
	}
}
