package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// maxBehaviorBindingDepth bounds the chain of names an `exhibit`/`perform`
// declaration is followed through to the element stating the body, so a binding
// that names itself is reported rather than followed forever.
const maxBehaviorBindingDepth = 32

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
	ID            int64                    // unique identity
	Type          *symbols.Symbol          // the def/usage symbol this instantiates
	FeatureValues map[string]*FeatureValue // feature name → feature value
	// classifiers are the features this object was held as a value of since it
	// was materialized, each of which classifies it further (see classify.go).
	classifiers []*symbols.Symbol
	// Ends are the ends of the connector this object materializes, in declaration
	// order, and nil for an object that is no connector. A named end also reads
	// through the feature value of that name; the order is what an end with no name of its
	// own is identified by.
	Ends []ConnectorEnd

	// anonymous holds, per anonymous connector declared, the object it materialized
	// to, 0 for one not materialized yet; nil until any is asked for.
	anonymous []int64

	// keptAnonymous holds the identities those objects had before a carry-over, by
	// the declaration each was of, until the one materialized again here takes it back.
	keptAnonymous []keptAnonymous

	// keptConnectors holds, per feature value of a named connector, the identity the object
	// of it had before a carry-over, which the one materialized again takes back.
	keptConnectors map[*FeatureValue]int64

	// behaviors are the executions of the behaviors the object's type exhibits or
	// performs, in declaration order, each bound to this object's identity.
	behaviors []*ObjectBehavior

	// owner is the object holding this one as the feature value named ownerFeature,
	// nil for an object no other holds. A nested object reaches its siblings
	// through it.
	owner        *Instance
	ownerFeature string

	// explicit marks an object a caller asked for by name, which stands on its
	// own even where its usage is a feature of a type.
	explicit bool

	// owed are the derived values this object took from its shape before
	// materializing all their derivations read (see shared_default.go).
	owed []owedDefault

	// nested are the tails of the nested redefinitions applying below this
	// object, each the chain rest a member of it applies (see nested_redefinition.go).
	nested []pendingRedefinition
}

// Owner answers the object holding this one and the feature of it that does, or
// nil and "" for an object no other holds.
func (inst *Instance) Owner() (*Instance, string) {
	return inst.owner, inst.ownerFeature
}

// keepConnector remembers the identity the object of a named connector feature value had,
// so the one materialized again against the new declarations keeps it.
func (inst *Instance) keepConnector(fv *FeatureValue, id int64) {
	if inst.keptConnectors == nil {
		inst.keptConnectors = make(map[*FeatureValue]int64)
	}
	inst.keptConnectors[fv] = id
}

// Feature value holds the runtime value(s) for one feature.
type FeatureValue struct {
	Feature        *EffectiveFeature
	body           *bodyCell
	Value          Value // scalar feature value (multiplicity [1])
	Values         Value // collection feature value (Sequence or Set)
	Materialized   bool  // lazy flag: has this feature value been instantiated?
	Written        bool  // a run assigned this value, so no default derives it again
	BindingDerived bool  // value came from binding propagation rather than a write
	// Assumed marks a value fixed by nothing but the feature's multiplicity: its
	// minimum materialized, or contributions short of its maximum.
	Assumed bool
	// dependents are the derived values that read this one, to unmaterialize when
	// it changes; nil until one does (see dependents.go).
	dependents []*FeatureValue
	// reads are the feature values a `=` value listed itself on, to delist from when
	// derived again; nil until it is derived.
	reads []*FeatureValue
	// changing is set while a write to this value is under way, so a write nested
	// in it counts as part of it (see beforeWrite).
	changing bool
	// intrinsic marks a value the feature's declarations alone materialized, which
	// every occurrence of the shape holds alike (see shared_default.go).
	intrinsic bool
}

// HeldValue is the value the feature value reads as: its collection when the feature is
// multi-valued, otherwise its scalar.
func (s *FeatureValue) HeldValue() Value {
	if s.Values.Kind != ValInvalid {
		return s.Values
	}
	return s.Value
}

// ReadValue is what the feature value reads as in an expression: what it holds, or the
// empty sequence when it holds nothing and its multiplicity admits that. A required
// feature holding nothing is uninitialized.
func (s *FeatureValue) ReadValue(name string) (Value, error) {
	value := s.HeldValue()
	if value.Kind != ValInvalid {
		return value, nil
	}
	if lower := s.Feature.Multiplicity.Lower; lower.Known && !lower.Infinite && lower.Value == 0 {
		return (*Context)(nil).collectionOf(s.Feature, nil), nil
	}
	return Value{}, fmt.Errorf("%w: %s", errUninitializedFeatureValue, name)
}

// UnsetText is how every surface spells a feature value that holds no value: a valueless
// feature of a value type, whose instances are values rather than objects.
const UnsetText = "<unset>"

// HoldsNoValue reports whether a value is an object materialized for a valueless
// feature of a value type. Such an object has no feature that could hold a value
// and is no value itself (KerML: a DataType classifies values), so it reads as
// unset rather than as an object.
func (ctx *Context) HoldsNoValue(val Value) bool {
	if val.Kind != ValInstance {
		return false
	}
	inst, ok := ctx.instances[val.Instance]
	return ok && semantics.IsValueType(inst.Type) && !ctx.shapeHoldsValue(inst.Type)
}

// shapeHoldsValue reports whether an object of typ carries a value of its own: a record's
// features, or a field the model adds to a value-held type (`:>> mRefs = (m, m)`, `label`).
func (ctx *Context) shapeHoldsValue(typ *symbols.Symbol) bool {
	features := ctx.FeaturesOf(typ)
	if len(features) == 0 {
		return false
	}
	if !ctx.model.semantics.ValueHeld(typ) {
		return true
	}
	usages := ctx.usageAndSpecializedUsages(typ)
	for _, feat := range features {
		if !holdsRecordField(feat.Symbol) {
			continue
		}
		// A library usage's own body (`:>> mRefs default (m, m, m)`) describes the particular
		// frame it names, for the usage and for a usage specializing it (`attribute local :> …`).
		if !ctx.libraryDeclared(feat.Symbol) || usages[feat.OwnerType] {
			return true
		}
	}
	return false
}

// usageAndSpecializedUsages is typ, when it is a usage, with every usage it
// directly specializes (`attribute local :> ...Frame;`); a definition contributes nothing.
func (ctx *Context) usageAndSpecializedUsages(typ *symbols.Symbol) map[*symbols.Symbol]bool {
	usages := map[*symbols.Symbol]bool{}
	if usageSymbol(typ) {
		usages[typ] = true
	}
	for _, sup := range ctx.model.semantics.DirectSupertypes(typ) {
		if usageSymbol(sup) {
			usages[sup] = true
		}
	}
	return usages
}

// usageSymbol reports whether sym declares a usage — a feature — rather than a
// definition; a KerML classifier is parsed as a usage node but classifies.
func usageSymbol(sym *symbols.Symbol) bool {
	_, ok := sym.Decl.(*ast.Usage)
	return ok && !sym.Kind.IsDefinition()
}

// noValueError reports the read of node, which found the unset value val, naming
// the feature that holds it.
func (ctx *Context) noValueError(val Value, node ast.Node) *NoValueError {
	err := &NoValueError{Feature: conditionText(node)}
	if ref, ok := node.(*ast.FeatureReference); ok {
		err.Ref = ref.Name
	}
	if inst, ok := ctx.instances[val.Instance]; ok && inst.owner != nil {
		if fv, ok := inst.owner.FeatureValues[inst.ownerFeature]; ok && fv.Feature != nil {
			err.Symbol = fv.Feature.Symbol
		}
	}
	return err
}

// Instantiate materializes an instance of the given usage/definition symbol.
// Allocates ID, creates feature values per FeaturesOf(sym), evaluates default values,
// leaves composite features lazy, then starts the behaviors the type exhibits or
// performs and runs them to quiescence. Returns the instance or an error.
//
// Each call materializes a distinct object with an identity and behaviors of its
// own. The object becomes what the usage denotes from then on, so an expression
// naming the usage or a feature path under it reads this object as it was run.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error) {
	return ctx.InstantiateRead(sym, nil)
}

// InstantiateRead is Instantiate with read run over the new object before it is
// returned, as one creation: a read that fails abandons the object with all it
// materialized, so a context under a bound is left as it was.
func (ctx *Context) InstantiateRead(sym *symbols.Symbol, read func(*Instance) error) (*Instance, error) {
	mark, attached := len(ctx.created), len(ctx.objectBehaviors)
	inst, err := ctx.materialize(sym, 0, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	// Registered before its behaviors start, so one of them naming the usage
	// reaches this object; a failed start abandons the occurrence with it.
	inst.explicit = true
	prior, hadPrior := ctx.occurrences[sym]
	restore := func() {
		if hadPrior {
			ctx.occurrences[sym] = prior
		}
	}
	if ctx.registersOccurrence(sym) {
		ctx.noteProbeUndo(func() {
			if hadPrior {
				ctx.occurrences[sym] = prior
			} else {
				delete(ctx.occurrences, sym)
			}
		})
		ctx.occurrences[sym] = []int64{inst.ID}
	}
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		restore()
		return nil, err
	}
	if read != nil {
		if err := read(inst); err != nil {
			ctx.abandonCreationSince(mark, attached)
			restore()
			return nil, err
		}
	}
	return inst, nil
}

// instantiateAs materializes an object under the given identity, falling back to
// the next one this context hands out when that identity is none or taken here.
func (ctx *Context) instantiateAs(sym *symbols.Symbol, id int64) (*Instance, error) {
	return ctx.instantiateOwnedBy(sym, id, nil, "")
}

// instantiateOwnedBy materializes an object held by owner as the feature value
// named feature. The owner is recorded before any behavior starts, so a behavior
// addressing a sibling reaches the object its owner holds rather than a second
// one.
func (ctx *Context) instantiateOwnedBy(sym *symbols.Symbol, id int64, owner *Instance, feature string) (*Instance, error) {
	// A creation that fails leaves none of the objects it reached behind, however
	// deeply nested or however a behavior of it addressed them.
	mark := len(ctx.created)
	inst, err := ctx.materialize(sym, id, owner, feature)
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
	return inst, nil
}

// newFeatureValue is the unmaterialized feature value inst holds for feat.
func (ctx *Context) newFeatureValue(inst *Instance, feat *EffectiveFeature) *FeatureValue {
	fv := &FeatureValue{}
	ctx.initFeatureValue(inst, fv, feat)
	return fv
}

// initFeatureValue makes fv the unmaterialized feature value inst holds for feat; an admitted constant
// default is folded eagerly, any other default is left for GetFeatureValue to evaluate and report.
func (ctx *Context) initFeatureValue(inst *Instance, fv *FeatureValue, feat *EffectiveFeature) {
	*fv = FeatureValue{Feature: feat}
	if ctx.valueBinds(feat) && feat.Scalar() && !ctx.model.semantics.IsVariationFeature(feat.Symbol) &&
		ctx.restatedInValuedBody(feat) == "" && !ctx.defaultYieldsToSubsetters(inst, feat) && !ctx.portionReadsWhole(inst, feat) {
		if semVal, ok := ctx.model.semantics.EvalWithin(feat.DefaultValue, ctx.maxIntegerBits); ok {
			val := Value{Kind: ValConst, Const: semVal}
			if ctx.checkDefault(inst, fv, feat.Name, &val, admitDeclared) == nil {
				fv.Value = val
				fv.Materialized, fv.intrinsic = true, true
			}
		}
	}
}

// defaultYieldsToSubsetters reports whether feat's `default` may be superseded by a
// feature of one of inst's types subsetting it, so it must wait to be read rather than be folded.
func (ctx *Context) defaultYieldsToSubsetters(inst *Instance, feat *EffectiveFeature) bool {
	if !feat.defaultIsFallback() {
		return false
	}
	for _, typ := range inst.types() {
		if len(ctx.SubsettingFeatures(nil, typ, feat.Name)) > 0 {
			return true
		}
	}
	return false
}

// unfoldSubsettedDefaults reopens the folded fallback defaults of inst that the
// features of classifier typ subset, so the next read takes their contributions.
func (ctx *Context) unfoldSubsettedDefaults(inst *Instance, typ *symbols.Symbol, features []EffectiveFeature) {
	for i := range features {
		if features[i].Symbol == nil {
			continue
		}
		for _, name := range ctx.declaredSubsettedNames(features[i].Symbol, typ) {
			fv, ok := inst.FeatureValues[name]
			if !ok || !fv.Materialized || fv.Written || !ctx.valueBinds(fv.Feature) || !fv.Feature.defaultIsFallback() {
				continue
			}
			ctx.noteProbeWrite(fv)
			fv.Value, fv.Values, fv.Materialized, fv.intrinsic = Value{}, Value{}, false, false
			ctx.invalidateDependents(fv)
		}
	}
}

// materialize builds the object and its feature values and registers it, before
// any behavior of it starts: an entry action reads the object's declared
// defaults, and a behavior can already reach the object it belongs to. It is
// held by owner as the feature named feature, or by nothing when owner is nil; a
// holder records the object it holds before those behaviors run, so one
// addressing it back through its holder reaches this object rather than a second.
func (ctx *Context) materialize(sym *symbols.Symbol, id int64, owner *Instance, feature string) (*Instance, error) {
	defer ctx.beginRun()()

	// Check step limit (I3)
	if err := ctx.incrementStep(); err != nil {
		return nil, err
	}
	if err := ctx.instanceRoom(); err != nil {
		return nil, err
	}

	if _, taken := ctx.instances[id]; taken || id <= 0 {
		id = ctx.allocateID()
	}
	ctx.claimID(id)

	// Get effective features
	features := ctx.FeaturesOf(sym)

	// A nested redefinition written on a type of this object's owner redefines a
	// feature below it: what applies on this object overrides its shape now, and
	// what applies below carries on it (see nested_redefinition.go).
	pending := ctx.pendingNestedRedefinitions(owner, feature)
	// The chains the object's own type and what it specializes declare reach
	// its members the same way, so a valued one governs an inherited bound
	// value (GovernedByChain) before the member materializes.
	for _, src := range append([]*symbols.Symbol{sym}, ctx.model.semantics.MemberSources(sym)...) {
		for _, nr := range ctx.model.semantics.NestedRedefinitionsOf(src) {
			if len(nr.Path) > 1 {
				pending = append(pending, pendingRedefinition{rest: nr.Path, sym: nr.Feature})
			}
		}
	}

	// Create instance
	inst := &Instance{
		ID:            id,
		Type:          sym,
		FeatureValues: make(map[string]*FeatureValue, len(features)),
		owner:         owner,
		ownerFeature:  feature,
	}
	if len(pending) > 0 {
		features, inst.nested = ctx.applyNestedRedefinitions(sym, features, pending)
	}
	features = ctx.withConjoinedTypes(inst, owner, feature, features)
	features = ctx.withBoundMembers(inst, owner, feature, features)

	// Create feature value for each feature, allocated as one block.
	values := make([]FeatureValue, len(features))
	for i := range features {
		fv := &values[i]
		ctx.initFeatureValue(inst, fv, &features[i])
		inst.FeatureValues[features[i].Name] = fv
	}

	// A redefining feature declares the feature it redefines again, so the two
	// names read one feature value, under every type of the object at once.
	if err := ctx.aliasRedefinedFeatureValuesOf(inst, inst.types(), nil, features); err != nil {
		return nil, err
	}
	ctx.bindImplicitSatisfactionSubject(inst, owner)

	// Register instance
	ctx.registerInstance(inst)
	ctx.beginLife(inst)

	if ctx.trace != nil {
		ctx.trace.recordObjectMaterialized(SymbolText(sym), inst.ID)
	}

	return inst, nil
}

// occurrenceOf returns the object a usage denotes, materializing it once: a part
// declared in a package names one occurrence, so reading its features twice
// reads the same object.
// beganWithRun dates the object a package-level usage denotes from the run's
// first activation: it exists throughout the run, not from its first read.
func (ctx *Context) beganWithRun(inst *Instance) {
	l := ctx.lives[inst.ID]
	if l.began <= 1 {
		return
	}
	// reached stays where the read materialized it, so `create` can still start it there.
	l.began, l.beganAt = 1, 0
	ctx.lives[inst.ID] = l
	ctx.livesChanged()
}

func (ctx *Context) occurrenceOf(sym *symbols.Symbol) (*Instance, error) {
	if objs, bound, err := ctx.namespaceBoundObjects(sym); err != nil {
		return nil, err
	} else if bound && len(objs) == 1 {
		return objs[0], nil
	} else if bound {
		return nil, fmt.Errorf("usage %s: %w: denotes %d occurrences, not one", SymbolText(sym), ErrMultiplicityViolation, len(objs))
	}
	if objs, subsetted, err := ctx.namespacedSubsetObjects(sym); err != nil {
		return nil, err
	} else if subsetted && len(objs) == 1 {
		return objs[0], nil
	} else if subsetted {
		return nil, fmt.Errorf("usage %s: %w: denotes %d occurrences, not one", SymbolText(sym), ErrMultiplicityViolation, len(objs))
	}
	// The occurrence is recorded before its behaviors start, so a behavior that
	// reaches the usage it belongs to reads this object rather than a second one.
	mark := len(ctx.created)
	inst, err := ctx.materializeMember(sym, 0, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	ctx.occurrences[sym] = []int64{inst.ID}
	if owner := sym.Owner(); owner == nil || owner.Kind == symbols.SymbolPackage {
		ctx.beganWithRun(inst)
	}
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
	return inst, nil
}

// occurrencesOf returns the objects a namespace-level usage of several occurrences denotes,
// materializing its lower bound once, in declaration order, as a nested collection's is.
func (ctx *Context) occurrencesOf(sym *symbols.Symbol) ([]*Instance, error) {
	if objs, bound, err := ctx.namespaceBoundObjects(sym); err != nil {
		return nil, err
	} else if bound {
		return objs, nil
	}
	if objs, subsetted, err := ctx.namespacedSubsetObjects(sym); err != nil {
		return nil, err
	} else if subsetted {
		return objs, nil
	}
	count, err := ctx.lowerBoundCount(ctx.featureMultiplicity(sym, ctx.findOwnerType(sym)), 0, SymbolText(sym))
	if err != nil {
		return nil, err
	}
	made, lazy := count, ctx.holdsLazily(sym, count)
	if lazy {
		made = 0
	}
	// The whole collection is recorded before any of its objects starts, so a behavior
	// reading the usage back reads the objects it denotes; a failure leaves none behind.
	release := ctx.elementScope()
	if err := ctx.chargeElements(made); err != nil {
		release()
		return nil, err
	}
	mark := len(ctx.created)
	members, err := ctx.materializeMembers(sym, int(made), nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		release()
		return nil, err
	}
	ids := make([]int64, len(members))
	for i, inst := range members {
		ids[i] = inst.ID
	}
	if lazy {
		seq, err := ctx.withRequired(nil, sym, nil, "", count-made)
		if err != nil {
			ctx.abandonInstancesSince(mark)
			release()
			return nil, err
		}
		ctx.recordOccurrenceTail(sym, seq.required)
	}
	ctx.occurrences[sym] = ids
	if err := ctx.startClassifierBehaviorsOf(members, mark); err != nil {
		release()
		return nil, err
	}
	return members, nil
}

// recordOccurrenceTail records r as the members sym denotes past its recorded occurrences.
func (ctx *Context) recordOccurrenceTail(sym *symbols.Symbol, r *requiredMembers) {
	prior, had := ctx.occurrenceTails[sym]
	ctx.occurrenceTails[sym] = r
	ctx.noteProbeUndo(func() {
		if had {
			ctx.occurrenceTails[sym] = prior
		} else {
			delete(ctx.occurrenceTails, sym)
		}
	})
}

// occurrenceTail is the members sym denotes past the occurrences recorded for it, when its
// lower bound is held lazily.
func (ctx *Context) occurrenceTail(sym *symbols.Symbol) *requiredMembers {
	r, ok := ctx.occurrenceTails[sym]
	if !ok {
		return nil
	}
	if live, ok := ctx.requiredOf(r.first); !ok || live != r {
		return nil
	}
	return r
}

// everyOccurrence is occurrencesOf with every member a lazily held lower bound denotes made,
// the work of reaching every one, charged to the element budget.
func (ctx *Context) everyOccurrence(sym *symbols.Symbol) ([]*Instance, error) {
	members, err := ctx.occurrencesOf(sym)
	if err != nil {
		return nil, err
	}
	r := ctx.occurrenceTail(sym)
	if r == nil {
		return members, nil
	}
	seq := &Sequence{elements: make([]Value, len(members)), required: r}
	if err := ctx.makeRequired(seq); err != nil {
		return nil, err
	}
	out := slices.Clone(members)
	for i := int64(0); i < r.count; i++ {
		out = append(out, ctx.instances[r.first+i])
	}
	return out, nil
}

// denotedObjects is the objects a namespace usage carrying no value denotes for the run: one
// occurrence, its lower bound of several, none when it admits none; a port denotes no object.
func (ctx *Context) denotedObjects(sym *symbols.Symbol) ([]*Instance, error) {
	switch {
	case ctx.namesOneObject(sym):
		inst, err := ctx.occurrenceOf(sym)
		if err != nil {
			return nil, fmt.Errorf("usage %s: %w", SymbolText(sym), err)
		}
		return []*Instance{inst}, nil
	case ctx.namesObjects(sym):
		// The run denotes the lower bound of a count it can materialize; a bound the
		// model does not evaluate leaves nothing to denote beyond what binds the usage.
		if mult := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym)); !mult.Upper.Known && !ctx.namespaceValued(sym) {
			if !ctx.optionalValueless(sym) {
				return nil, ctx.undenotedUsage(sym)
			}
			return nil, nil
		}
		members, err := ctx.everyOccurrence(sym)
		if err != nil {
			return nil, fmt.Errorf("usage %s: %w", SymbolText(sym), err)
		}
		return members, nil
	case ctx.optionalValueless(sym):
		// Of itself an optional usage denotes nothing, but a binding connector
		// may have bound it to another usage's value.
		if objs, bound, err := ctx.namespaceBoundObjects(sym); err != nil {
			return nil, err
		} else if bound {
			return objs, nil
		}
		return nil, nil
	}
	return nil, ctx.undenotedUsage(sym)
}

// denotedValue is what a usage carrying no value reads as: its object, the collection of its
// objects, or undetermined of its count where the model leaves its lower bound unmet.
func (ctx *Context) denotedValue(sym *symbols.Symbol) (Value, error) {
	// A reader of another document's scope tree holds its own symbol for the
	// usage; its occurrences are recorded under the declaring tree's.
	sym = ctx.declaredSymbol(sym)
	if !ctx.namesObjects(sym) {
		inst, err := ctx.occurrenceOf(sym)
		if err != nil {
			return Value{}, fmt.Errorf("usage %s: %w", SymbolText(sym), err)
		}
		return ctx.objectValue(inst)
	}
	mult := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym))
	if val, decided, err := ctx.openDenotedValue(sym, mult); err != nil {
		return Value{}, fmt.Errorf("usage %s: %w", SymbolText(sym), err)
	} else if decided {
		return val, nil
	}
	members, err := ctx.occurrencesOf(sym)
	if err != nil {
		return Value{}, fmt.Errorf("usage %s: %w", SymbolText(sym), err)
	}
	tail := ctx.occurrenceTail(sym)
	held := int64(len(members))
	if tail != nil {
		held += tail.count
	}
	if held < fewestOf(mult) {
		spelled := ctx.QualifiedName(sym)
		return undeterminedFeatureValue(openCountReason(spelled, mult), mult, sym), nil
	}
	elements := make([]Value, 0, len(members))
	for _, inst := range members {
		val, err := ctx.objectValue(inst)
		if err != nil {
			return Value{}, err
		}
		elements = append(elements, val)
	}
	if tail != nil {
		return NewSequenceValue(&Sequence{elements: elements, required: tail}), nil
	}
	return ctx.declaredCollection(sym, sequenceOf(elements)), nil
}

// liveOccurrences is the objects recorded as what sym denotes, while every one of them lives.
func (ctx *Context) liveOccurrences(sym *symbols.Symbol) ([]*Instance, bool) {
	ids, ok := ctx.occurrences[sym]
	if !ok {
		return nil, false
	}
	out := make([]*Instance, 0, len(ids))
	for _, id := range ids {
		inst, live := ctx.instances[id]
		if !live {
			return nil, false
		}
		out = append(out, inst)
	}
	return out, true
}

// denotesOccurrence reports whether inst is one of the objects its usage denotes.
func (ctx *Context) denotesOccurrence(inst *Instance) bool {
	if r := ctx.occurrenceTail(inst.Type); r != nil && r.holds(inst.ID) {
		return true
	}
	return slices.Contains(ctx.occurrences[inst.Type], inst.ID)
}

// OccurrenceUsage is the qualified name of the declared usage inst is an
// occurrence of, or "" for an object materialized any other way.
func (ctx *Context) OccurrenceUsage(inst *Instance) string {
	if inst == nil || !ctx.denotesOccurrence(inst) {
		return ""
	}
	return ctx.QualifiedName(inst.Type)
}

// isOccurrenceUsage reports whether sym declares a usage that is an occurrence:
// a part, item or individual, which is a thing with features rather than a
// value, so a chain through it reads the features of that thing.
func isOccurrenceUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value != nil {
		return false
	}
	return objectUsageKind(usage.Kind)
}

// objectUsageKind reports a usage kind whose values are objects: the occurrence usages
// (SysML v2 §7.9), cases, views, viewpoints, concerns and renderings among them (§7.21–7.26).
func objectUsageKind(kind ast.UsageKind) bool {
	switch kind {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence, ast.UsageIndividual,
		ast.UsageCase, ast.UsageUseCase, ast.UsageSatisfy,
		ast.UsageView, ast.UsageViewpoint, ast.UsageConcern, ast.UsageFramedConcern,
		ast.UsageRendering, ast.UsageViewRendering,
		ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation:
		return true
	}
	return false
}

// namesOneObject reports whether a usage denotes one object of its own — which
// a name evaluates to and a feature chain reads members from — rather than a
// value: an occurrence, or a structured value whose own features carry it.
func (ctx *Context) namesOneObject(sym *symbols.Symbol) bool {
	if sym == nil || !ctx.occursOnce(sym) || ctx.optionalValueless(sym) {
		return false
	}
	// A variation classifies its variants abstractly, so it is no object of
	// itself: it holds nothing until it is bound to one.
	if ctx.model.semantics.IsVariationFeature(sym) {
		return false
	}
	return isOccurrenceUsage(sym) || ctx.namesStructuredValue(sym)
}

// namesObjects reports whether a usage denotes several objects of its own: an occurrence
// usage a namespace declares of a collection multiplicity whose lower bound the model fixes.
func (ctx *Context) namesObjects(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil || !namespaceScope(sym.OwnerScope) || !isOccurrenceUsage(sym) {
		return false
	}
	if ctx.occursOnce(sym) || ctx.model.semantics.IsVariationFeature(sym) {
		return false
	}
	mult := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym))
	return mult.Lower.Known && !mult.Lower.Infinite
}

// registersOccurrence reports whether an object materialized for a usage is the one a
// later read reaches: the object it denotes, or a unit's, read through the unit's reference.
func (ctx *Context) registersOccurrence(sym *symbols.Symbol) bool {
	if ctx.namesOneObject(sym) {
		return true
	}
	return ctx.declaresUnit(sym) && ctx.occursOnce(sym)
}

// namesStructuredValue reports whether a usage carrying no value of its own is
// typed by a structured value — a non-scalar `attribute def` with features — so
// it holds those features rather than one scalar value.
func (ctx *Context) namesStructuredValue(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value != nil || usage.Kind != ast.UsageAttribute {
		return false
	}
	typ := ctx.extractType(sym)
	if typ == nil || ctx.model.semantics.PrimTypeOf(typ) != semantics.PrimUnknown {
		return false
	}
	return ctx.shapeHoldsValue(sym)
}

// occursOnce reports whether a usage names at most one occurrence; several occurrences, or
// a count the model does not fix, are a collection rather than one object to read features from.
func (ctx *Context) occursOnce(sym *symbols.Symbol) bool {
	mult := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym))
	return mult.Upper.Known && !mult.Upper.Infinite && mult.Upper.Value <= 1
}

// optionalValueless reports whether a usage or subject declares no value and a
// lower bound of zero: it holds only what is contributed to it, so of itself it is empty.
func (ctx *Context) optionalValueless(sym *symbols.Symbol) bool {
	switch sym.Decl.(type) {
	case *ast.Usage, *ast.SubjectMember:
	default:
		return false
	}
	if ctx.extractDefaultValue(sym) != nil {
		return false
	}
	return ctx.admitsNoValue(sym)
}

// admitsNoValue reports whether the feature's effective multiplicity has a lower
// bound of zero, so that holding no value at all is within it.
func (ctx *Context) admitsNoValue(sym *symbols.Symbol) bool {
	lower := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym)).Lower
	return lower.Known && !lower.Infinite && lower.Value == 0
}

// checkDefault reports a value the feature does not admit: a count outside its
// multiplicity (1..1 when none is declared) or an element outside its type.
func (ctx *Context) checkDefault(inst *Instance, fv *FeatureValue, name string, val *Value, how admission) error {
	what := func() string { return fmt.Sprintf("feature value %s.%s", inst.Type.Name, name) }
	return ctx.checkAdmits(fv.Feature, what, val, how)
}

// checkAdmits reports a value the feature does not admit, by count where a
// multiplicity is stated for it, by type or by uniqueness, naming the value as what spells it.
func (ctx *Context) checkAdmits(feat *EffectiveFeature, what func() string, val *Value, how admission) error {
	if msg := feat.Multiplicity.HeldViolation(heldCountOf(val)); msg != "" && !feat.MultiplicityDefaulted {
		return fmt.Errorf("%s: %w: %s", what(), ErrMultiplicityViolation, msg)
	}
	if err := ctx.checkWriteType(feat.DeclScope(), what, feat.Type, val, how); err != nil {
		return err
	}
	if msg := ctx.uniquenessRefusal(feat.Unique, feat.HoldsSet, val); msg != "" {
		return fmt.Errorf("%s: %w: %s", what(), ErrUniquenessViolation, msg)
	}
	return nil
}

// heldBy is the declaration whose values the feature's are: the feature itself,
// or its type for a feature the run time made up.
func (feat *EffectiveFeature) heldBy() *symbols.Symbol {
	if feat.Symbol != nil {
		return feat.Symbol
	}
	return feat.Type
}

// admitted is the value an admitted val is stored as: a collection for a multi-valued
// feature, its elements charged, the objects a declared value holds classified by the feature.
func (ctx *Context) admitted(feat *EffectiveFeature, val Value, how admission) (Value, error) {
	if val.Kind == ValUndetermined {
		// A value the model leaves open is held as it is: it is of no fixed count to collect.
		return val, nil
	}
	if !feat.Scalar() && (val.Kind != ValSequence && val.Kind != ValSet || feat.HoldsSet != (val.Kind == ValSet)) {
		// A multi-valued feature holds a collection however it was written, so a
		// single value written to one is that collection's one element.
		elements := elementsOf(val)
		if err := ctx.chargeElements(int64(len(elements))); err != nil {
			return Value{}, err
		}
		val = ctx.collectionOf(feat, elements)
	} else if feat.Scalar() {
		val = soleElement(val)
	} else if err := ctx.chargeShaped(val); err != nil {
		return Value{}, err
	}
	if how == admitDeclared {
		if err := ctx.classifyHeld(feat.heldBy(), val); err != nil {
			return Value{}, err
		}
		val = ctx.classifiedFrame(feat.Symbol, val)
	}
	return val, nil
}

// GetFeatureValue retrieves the feature value for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet. A feature value that could
// not be materialized is marked as such — it unwraps to ErrFeatureValueMaterialization —
// so a caller can tell it from any other failure to evaluate, whatever the
// expression it surfaced through.
func (inst *Instance) GetFeatureValue(ctx *Context, name string) (*FeatureValue, error) {
	return inst.getFeatureValue(ctx, name, nil)
}

// openPopulation is where a read that stops short of making up a collection's lower bound
// leaves what it certainly holds: its subsetters' values, and the fewest an open one holds.
type openPopulation struct {
	Stopped     bool
	Contributed []Value
	AtLeast     int64
	Fixed       int // the leading Contributed whose positions the model fixes
}

// openFeatureValue is GetFeatureValue for a model-level read: a collection whose count
// the model leaves open is not made up to its lower bound, and open reports that.
func (inst *Instance) openFeatureValue(ctx *Context, name string) (*FeatureValue, *openPopulation, error) {
	open := &openPopulation{}
	fv, err := inst.getFeatureValue(ctx, name, open)
	return fv, open, err
}

func (inst *Instance) getFeatureValue(ctx *Context, name string, open *openPopulation) (*FeatureValue, error) {
	if err := ctx.checkNotDestroyed(inst); err != nil {
		return nil, err
	}
	if _, ok := inst.FeatureValues[name]; !ok {
		if fv, derived, err := ctx.kernelCollectionRead(inst, name, open); derived {
			return fv, err
		}
		// Naming no feature value of the object is no materialization of one.
		return nil, fmt.Errorf("%w: feature %q not found in instance %d (type %s)", ErrNoSuchFeature, name, inst.ID, inst.Type.Name)
	}
	if fv, err := inst.shotFeatureValue(ctx, name); fv != nil || err != nil {
		return fv, err
	}
	fv, err := inst.materializeFeatureValue(ctx, name, open)
	if err != nil {
		return nil, &FeatureValueError{Err: err, Instance: inst, Feature: name}
	}
	return fv, nil
}

// SetFeatureValue writes a value to the named feature value of the object, which is how a
// behavior the object performs updates the object's own state. The value must
// conform to the multiplicity governing the feature; a feature the object does
// not have is reported rather than added. A feature declared `constant` or
// `derived`, or redefining or subsetting one that is, is refused as
// errReadOnlyFeature and keeps what it held: a constant feature does not change
// over the lifetime of its featuring occurrence, and a derived one has the
// values the model determines (KerML 1.0 §8.3.3.3.4, Feature::isConstant and
// Feature::isDerived).
func (inst *Instance) SetFeatureValue(ctx *Context, name string, value Value) error {
	fv, err := inst.writableFeatureValue(ctx, name)
	if err != nil {
		return err
	}
	if fv.Feature.Symbol != nil {
		what := func() string { return fmt.Sprintf("write %s of object #%d (%s)", name, inst.ID, SymbolText(inst.Type)) }
		if err := ctx.readOnlyRefusal(what, name, fv.Feature.Symbol); err != nil {
			return err
		}
	}
	return inst.storeFeatureValue(ctx, fv, name, value)
}

// BindFeatureValue writes a value to the named feature value of the object as
// the model determines it rather than as a behavior changes it: the object's
// initial values, the seeding of a performance occurrence with what its run
// already holds, a parameter bound at a call, a binding's other end, a message's
// payload. It answers to the declaration as SetFeatureValue does, but a constant
// or derived feature is not refused, since that is how such a feature gets its
// values.
func (inst *Instance) BindFeatureValue(ctx *Context, name string, value Value) error {
	fv, err := inst.writableFeatureValue(ctx, name)
	if err != nil {
		return err
	}
	return inst.storeFeatureValue(ctx, fv, name, value)
}

// writableFeatureValue is the feature value a write of name lands in.
func (inst *Instance) writableFeatureValue(ctx *Context, name string) (*FeatureValue, error) {
	if err := ctx.checkNotDestroyed(inst); err != nil {
		return nil, err
	}
	fv, ok := inst.FeatureValues[name]
	if !ok {
		return nil, fmt.Errorf("%w: feature %q not found in instance %d (type %s)", ErrNoSuchFeature, name, inst.ID, inst.Type.Name)
	}
	return fv, nil
}

// storeFeatureValue stores value in fv once it conforms to the feature's declaration.
func (inst *Instance) storeFeatureValue(ctx *Context, fv *FeatureValue, name string, value Value) error {
	// Checked before the write, so a value the feature does not admit leaves it
	// holding what it held.
	if err := ctx.checkDefault(inst, fv, name, &value, admitWritten); err != nil {
		return err
	}
	value, err := ctx.admitted(fv.Feature, value, admitWritten)
	if err != nil {
		return err
	}
	endWrite := ctx.beginFeatureWrite(fv)
	defer endWrite()
	return ctx.storedBeforeStarting(func() error {
		if err := ctx.holdWritten(inst, fv, value); err != nil {
			return fmt.Errorf("feature %s.%s: %w", inst.Type.Name, name, err)
		}
		ctx.noteProbeWrite(fv)
		before := ctx.beforeWrite(fv)
		held := fv.HeldValue()
		if fv.Feature.Scalar() {
			fv.Value = value
			fv.Values = Value{}
		} else {
			fv.Values = value
			fv.Value = Value{}
		}
		fv.Materialized, fv.Written = true, true
		fv.BindingDerived, fv.Assumed, fv.intrinsic = false, false, false
		ctx.unshareTraces()
		ctx.afterWrite(fv, before)
		ctx.adoptOccurrenceWrite(inst, fv)
		ctx.adoptBodyCellValue(fv)
		ctx.releaseStandIns(held)
		return nil
	})
}

// releaseStandIns drops the objects a value-typed feature materialized to stand for its
// unspecified values once a write gives it some: a data value has no identity of its own.
func (ctx *Context) releaseStandIns(held Value) {
	switch held.Kind {
	case ValInstance:
		ctx.releaseStandIn(held)
	case ValSequence, ValSet, ValArray:
		for _, elem := range listedElements(held) {
			ctx.releaseStandIn(elem)
		}
	}
}

// releaseStandIn drops elem when it is a stand-in a write just replaced that nothing names any more.
func (ctx *Context) releaseStandIn(elem Value) {
	if elem.Kind != ValInstance || !ctx.HoldsNoValue(elem) {
		return
	}
	standIn := ctx.instances[elem.Instance]
	if len(standIn.behaviors) != 0 {
		return
	}
	named := map[int64]bool{standIn.ID: true}
	if ctx.featureValuesName(named) || ctx.runDataNames(named) {
		return
	}
	ctx.dropStandIn(standIn)
}

// featureValuesName reports whether any materialized feature value still names one of the objects.
func (ctx *Context) featureValuesName(named map[int64]bool) bool {
	for _, inst := range ctx.instances {
		for _, fv := range inst.FeatureValues {
			if fv.Materialized && namesAbandoned(fv, named) {
				return true
			}
		}
	}
	return false
}

// dropStandIn unregisters a stand-in, lifetime included; a journal rolled back puts it back.
func (ctx *Context) dropStandIn(standIn *Instance) {
	id := standIn.ID
	at := slices.Index(ctx.created, id)
	if at < 0 {
		return
	}
	l, lived := ctx.lives[id]
	ctx.forgetAbandoned(ctx.dropInstances(at, at+1, func(*Instance) bool { return true }))
	ctx.noteProbeUndo(func() {
		ctx.instances[id] = standIn
		ctx.created = slices.Insert(ctx.created, min(at, len(ctx.created)), id)
		if lived {
			ctx.lives[id] = l
		}
	})
}

// adoptOccurrenceWrite has every run over inst whose data mirrors fv hold what the
// write landed on it, before anything the write replaced is released.
func (ctx *Context) adoptOccurrenceWrite(inst *Instance, fv *FeatureValue) {
	if ctx.mirroringBodyCell != 0 || ctx.probes > 0 {
		return
	}
	mirrored := false
	for _, exec := range ctx.stateExecutors {
		if exec.occurrence == inst && exec.mirrorOccurrenceWrite(fv) {
			mirrored = true
		}
	}
	for _, exec := range ctx.actionExecutors {
		if exec.occurrence == inst {
			exec.mirrorOccurrenceWrite(fv)
		}
	}
	if mirrored {
		ctx.noteStateDataWrite()
	}
}

// runDataNames reports whether the data of a live run still names one of the objects.
func (ctx *Context) runDataNames(named map[int64]bool) bool {
	for _, exec := range ctx.stateExecutors {
		if valuesName(exec.stateData, named) {
			return true
		}
	}
	for _, exec := range ctx.actionExecutors {
		for _, perf := range exec.reachableFrames() {
			if valuesName(perf.data, named) {
				return true
			}
		}
	}
	return false
}

// valuesName reports whether any of vars names one of the objects.
func valuesName(vars map[string]Value, objects map[int64]bool) bool {
	for _, val := range vars {
		if namesAbandonedValue(val, objects) {
			return true
		}
	}
	return false
}

// materializeFeatureValue is GetFeatureValue's materialization: the feature value's value, evaluated and
// checked against the multiplicity governing its feature the first time it is read.
func (inst *Instance) materializeFeatureValue(ctx *Context, name string, open *openPopulation) (*FeatureValue, error) {
	fv := inst.FeatureValues[name]
	err := ctx.storedBeforeStarting(func() error {
		before := ctx.beforeWrite(fv)
		err := inst.materializeBoundOrIntrinsic(ctx, fv, name, open)
		ctx.afterWrite(fv, before)
		return err
	})
	if err != nil {
		return nil, err
	}
	ctx.noteRead(inst, fv)
	return fv, nil
}

// materializeBoundOrIntrinsic gives fv the value a binding determines, else the one
// its feature states, once.
func (inst *Instance) materializeBoundOrIntrinsic(ctx *Context, fv *FeatureValue, name string, open *openPopulation) error {
	if val, found, err := ctx.resolveBindingValue(inst, name); err != nil {
		return err
	} else if found {
		return ctx.assignBindingValue(inst, fv, name, val)
	} else if !fv.Materialized {
		_, err := inst.materializeIntrinsicValue(ctx, name, open)
		return err
	}
	return nil
}

// bindingContext is the object a feature's value is read against: the owner of inst for a
// case's or requirement's subject (KerML 1.0 §7.4.11, `subject subj default Case::result`).
func (ctx *Context) bindingContext(inst *Instance, feat *EffectiveFeature) *Instance {
	if feat == nil || !isSubjectUsage(feat.Symbol) {
		return inst
	}
	if owner, _ := inst.Owner(); owner != nil {
		return owner
	}
	return inst
}

// materializeFeatureValueIntrinsic evaluates a feature without following
// binding connectors; binding resolution calls it to inspect an endpoint.
func (inst *Instance) materializeFeatureValueIntrinsic(ctx *Context, name string) (*FeatureValue, error) {
	return inst.materializeIntrinsicValue(ctx, name, nil)
}

func (inst *Instance) materializeIntrinsicValue(ctx *Context, name string, open *openPopulation) (*FeatureValue, error) {
	fv := inst.FeatureValues[name]
	err := ctx.storedBeforeStarting(func() error {
		before := ctx.beforeWrite(fv)
		_, err := inst.materializeIntrinsic(ctx, fv, name, open)
		ctx.afterWrite(fv, before)
		return err
	})
	if err != nil {
		return nil, err
	}
	return fv, nil
}

// materializeIntrinsic evaluates fv from what the model states of its feature: a
// variation, a default, the members subsetting it, or the objects it holds. A non-nil
// open stops the read short of making up an open collection's lower bound.
func (inst *Instance) materializeIntrinsic(ctx *Context, fv *FeatureValue, name string, open *openPopulation) (*FeatureValue, error) {
	ctx.noteProbeWrite(fv)

	// A feature listing this one as a value, on this object or an owner reaching it by a
	// chain, classifies its objects: each is read first, and may have materialized this one.
	ctx.materializeHolders(inst, name)
	if fv.Materialized {
		return fv, nil
	}

	// A variation holds the variant it was bound to, and nothing until it is
	// bound: it classifies its variants abstractly, so it is no object of itself.
	if ctx.model.semantics.IsVariationFeature(fv.Feature.Symbol) {
		return inst.materializeVariation(ctx, fv, name)
	}

	// A portion holds the whole's value of a feature its own body does not value (portion.go).
	if held, err := inst.materializeFromWhole(ctx, fv, name); err != nil || held {
		return fv, err
	}

	// A bound value supplies the feature's own features, so a body restating one
	// of them states two values for it.
	if restated := ctx.restatedInValuedBody(fv.Feature); restated != "" {
		return nil, fmt.Errorf("feature value %s.%s: %w: %s", inst.Type.Name, name, errValuedFeatureRestated, restated)
	}
	if restated := ctx.restatedByNestedChain(inst, name, fv.Feature); restated != "" {
		return nil, fmt.Errorf("feature value %s.%s: %w: %s", inst.Type.Name, name, errValuedFeatureRestated, restated)
	}

	// A `default` applies only where nothing else populates the feature: the
	// members subsetting it do (KerML 1.0 §7.3.4.5).
	if ctx.valueBinds(fv.Feature) && fv.Feature.defaultIsFallback() {
		contributed, err := ctx.subsettingContributions(inst, name)
		if err != nil {
			return nil, err
		}
		if len(contributed) > 0 {
			return inst.holdContributed(ctx, fv, name, contributed)
		}
	}

	// A default that did not constant-fold is a derived value: evaluate it
	// against this instance, so that it sees the sibling feature values it refers to.
	// The feature holds what the default states, once that conforms to the
	// feature's multiplicity and type.
	if ctx.valueBinds(fv.Feature) {
		return inst.materializeDerived(ctx, fv, name)
	}

	// A collection an optional feature subsets fills its objects: it is read first.
	if err := ctx.materializeSubsettedCollections(inst, fv); err != nil {
		return nil, err
	}
	if fv.Materialized {
		return fv, nil
	}

	// A feature reference-subsetting another of this object holds that one's values.
	if held, err := inst.materializeReferenced(ctx, fv, name); err != nil || held {
		return fv, err
	}

	// A `default` the holding declaration inherits and a body governs over supplies
	// the features of the one object that body leaves unvalued (see default_body.go).
	if held, err := inst.materializeFromHolderDefault(ctx, fv, name); err != nil || held {
		return fv, err
	}

	// An abstract feature has no values of its own (KerML 1.0 §7.3.3.1) and an
	// optional one demands none: each, a connector included, holds only contributions —
	// unless the declaration's body binds a feature of the one object it then holds.
	if fv.Feature.HoldsOnlyContributions() && !ctx.bodyBindsAFeature(fv.Feature) && (symbols.IsAbstract(fv.Feature.Symbol) || ctx.model.semantics.IsConnectorObjectUsage(fv.Feature.Symbol) || ctx.CompositeTypeOf(fv.Feature) != nil) {
		// A binding stating a link makes an optional subsetter hold a value its other end holds.
		if composite := ctx.CompositeTypeOf(fv.Feature); composite != nil {
			if held, err := inst.materializeFromSubsetted(ctx, fv, name, composite); err != nil || held {
				return fv, err
			}
		}
		return inst.holdContributions(ctx, fv, name)
	}

	// A connector holds the features it connects at its ends rather than objects
	// of its own, so it is materialized from what the `connect` clause names.
	if ctx.model.semantics.IsConnectorObjectUsage(fv.Feature.Symbol) {
		if err := ctx.materializeConnectorFeatureValue(inst, fv, name); err != nil {
			return nil, err
		}
		return fv, nil
	}

	// Lazy instantiation: a composite feature holds objects of its own.
	if composite := ctx.CompositeTypeOf(fv.Feature); composite != nil {
		return inst.materializeComposite(ctx, fv, name, open, composite)
	}

	return fv, nil
}

// materializeVariation binds a variation to the variant its default selects.
func (inst *Instance) materializeVariation(ctx *Context, fv *FeatureValue, name string) (*FeatureValue, error) {
	if fv.Feature.DefaultValue == nil {
		return nil, fmt.Errorf("%w: %s.%s", ErrVariationUnselected, inst.Type.Name, name)
	}
	val, err := ctx.evalFeatureValueDefault(inst, fv, name)
	if err != nil {
		return nil, err
	}
	bound, err := ctx.bindVariation(fv.Feature, val, inst.ID)
	if err != nil {
		return nil, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
	}
	fv.Value = bound
	fv.Materialized, fv.intrinsic = true, false
	return fv, nil
}

// materializeDerived evaluates a default against this instance and holds what
// it states once that conforms to the feature's multiplicity and type.
func (inst *Instance) materializeDerived(ctx *Context, fv *FeatureValue, name string) (*FeatureValue, error) {
	if ctx.takeShared(inst, fv) {
		return fv, nil
	}
	return inst.holdDerived(ctx, fv, name, func() (Value, error) { return ctx.evalFeatureValueDefault(inst, fv, name) })
}

// holdDerived holds what derive states for the feature, recording what it reads, once
// that conforms to the feature's multiplicity and type.
func (inst *Instance) holdDerived(ctx *Context, fv *FeatureValue, name string, derive func() (Value, error)) (*FeatureValue, error) {
	val, clean, reads, err := ctx.deriveFeatureValue(inst, fv, derive)
	if err != nil {
		return nil, err
	}
	if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
		return nil, err
	}
	if val, err = ctx.holdDeclared(inst, fv, val); err != nil {
		return nil, err
	}
	ctx.noteProbeWrite(fv)
	if fv.Feature.Scalar() {
		fv.Value = val
	} else {
		fv.Values = val
	}
	fv.Materialized, fv.intrinsic = true, clean
	ctx.shareDerived(inst, fv, val, clean, reads)
	return fv, nil
}

// materializeComposite instantiates the objects a composite feature holds: one
// for a scalar, otherwise the members making up the collection's lower bound.
func (inst *Instance) materializeComposite(ctx *Context, fv *FeatureValue, name string, open *openPopulation, composite *symbols.Symbol) (*FeatureValue, error) {
	mult := fv.Feature.Multiplicity
	// A model-level read holds what the model declares: it makes up no member for a
	// lower bound the declared contributions leave unmet, and reports that instead.
	_, exact := mult.Exactly()
	if open != nil && !exact && !(fv.Feature.Scalar() && ctx.bodyBindsAFeature(fv.Feature)) {
		release := ctx.elementScope()
		contributed, atLeast, err := ctx.openSubsettingContributions(inst, name)
		release()
		if err != nil {
			return nil, err
		}
		held := int64(len(contributed))
		if !mult.Lower.Known || !mult.Upper.Known || max(atLeast, fewestOf(mult)) > held || semantics.IsBehaviorParameter(fv.Feature.Symbol) {
			open.Stopped, open.Contributed, open.AtLeast = true, contributed, atLeast
			return fv, nil
		}
	}

	if !mult.Upper.Known || !mult.Lower.Known {
		return nil, fmt.Errorf("cannot materialize feature %q with unknown multiplicity", name)
	}

	// A required feature subsetting another holds that one's values (KerML 1.0 §7.3.4.4).
	if held, err := inst.materializeFromSubsetted(ctx, fv, name, composite); err != nil || held {
		return fv, err
	}

	if !mult.Upper.Infinite && mult.Upper.Value == 1 {
		// Scalar: instantiate one, held by this feature before its behaviors
		// start, so one addressing it back reads the object held here.
		mark := len(ctx.created)
		childInst, err := ctx.materialize(composite, 0, inst, name)
		if err != nil {
			ctx.abandonInstancesSince(mark)
			return nil, err
		}
		fv.Value = Value{Kind: ValInstance, Instance: childInst.ID}
		fv.Materialized, fv.intrinsic = true, true
		if err := ctx.startClassifierBehaviors(childInst, mark); err != nil {
			return nil, err
		}
	} else if err := inst.materializeCompositeCollection(ctx, fv, name, composite); err != nil {
		return nil, err
	}
	fv.Materialized = true
	return fv, nil
}

// materializeCompositeCollection fills a composite collection: subsetting
// features' objects are members; optional subsetters with room, then anonymous
// objects, make up the lower bound. A failed read leaves nothing behind.
func (inst *Instance) materializeCompositeCollection(ctx *Context, fv *FeatureValue, name string, composite *symbols.Symbol) error {
	mult := fv.Feature.Multiplicity
	release := ctx.elementScope()
	contributed, err := ctx.subsettingContributions(inst, name)
	if err != nil {
		release()
		return err
	}

	count, err := ctx.lowerBoundCount(mult, len(contributed), fmt.Sprintf("feature %q", name))
	if err != nil {
		release()
		return err
	}
	// Past eagerLowerBound, the anonymous members beyond the first are held as required
	// members, made when reached, unless the optional subsetters could hold them all.
	lazy := !fv.Feature.HoldsSet && ctx.holdsLazily(composite, count) && ctx.subsetterRoom(inst, name) < count
	fill := count
	if lazy {
		fill = min(count, ctx.subsetterRoom(inst, name)+1)
	}

	// The whole collection is held before any of its objects starts, so a
	// behavior reading the feature back reads the objects held in it.
	if err := ctx.chargeElements(fill); err != nil {
		release()
		return err
	}
	mark := len(ctx.created)
	children, unfill, err := ctx.fillOptionalSubsetters(inst, name, int(fill))
	fail := func(err error) error {
		fv.Values, fv.Materialized, fv.intrinsic = Value{}, false, false
		ctx.abandonInstancesSince(mark)
		unfill()
		release()
		return err
	}
	if err != nil {
		return fail(err)
	}
	made, err := ctx.materializeMembers(composite, int(fill)-len(children), inst, name)
	if err != nil {
		return fail(err)
	}
	children = append(children, made...)
	elements := make([]Value, 0, len(contributed)+len(children))
	elements = append(elements, contributed...)
	for _, child := range children {
		elements = append(elements, Value{Kind: ValInstance, Instance: child.ID})
	}
	if lazy {
		seq, err := ctx.withRequired(elements, composite, inst, name, count-fill)
		if err != nil {
			return fail(err)
		}
		fv.Values = NewSequenceValue(seq)
	} else {
		fv.Values = ctx.collectionOf(fv.Feature, elements)
	}
	fv.Materialized = true
	fv.Assumed = semantics.IsBehaviorParameter(fv.Feature.Symbol) || count > 0 && mult.AdmitsMore(int64(len(contributed))+count)
	fv.intrinsic = ctx.subsettersDeclared(inst, name)
	if err := ctx.startClassifierBehaviorsOf(children, mark); err != nil {
		return fail(err)
	}
	return nil
}

// holdsLazily reports whether count objects of typ making up a lower bound are held as
// required members: more than eagerLowerBound of a type whose objects start no behavior,
// so making one when it is first reached is indistinguishable from making it now.
func (ctx *Context) holdsLazily(typ *symbols.Symbol, count int64) bool {
	if count <= eagerLowerBound {
		return false
	}
	return ctx.declarative || !ctx.runsBehaviors(typ, make(map[*symbols.Symbol]bool))
}

// lowerBoundCount is how many anonymous objects fill a collection to its lower bound
// beyond the held objects already counted. An unknown bound fixes no count, and an
// infinite one no finite number of values (KerML 1.0 §7.4.12), so neither is filled.
func (ctx *Context) lowerBoundCount(mult semantics.Range, held int, what string) (int64, error) {
	if !mult.Upper.Known || !mult.Lower.Known {
		return 0, fmt.Errorf("cannot materialize %s with unknown multiplicity", what)
	}
	if mult.Lower.Infinite {
		return 0, fmt.Errorf("%w for %s: a lower bound of * requires no finite number of values (KerML 1.0 §7.4.12 Multiplicities)", errInfiniteLowerBound, what)
	}
	return max(mult.Lower.Value-int64(held), 0), nil
}

// materializeMembers makes count objects of sym in order, the members a collection
// is filled with; the caller abandons what a failed one leaves behind.
func (ctx *Context) materializeMembers(sym *symbols.Symbol, count int, owner *Instance, feature string) ([]*Instance, error) {
	members := make([]*Instance, 0, count)
	for i := 0; i < count; i++ {
		inst, err := ctx.materializeMember(sym, 0, owner, feature)
		if err != nil {
			return nil, err
		}
		members = append(members, inst)
	}
	return members, nil
}

// materializeMember makes one object of sym under identity id (0 for the next), held by owner in
// feature, its behaviors left for the caller to start once it is recorded; a namespace-level
// connector usage's object is made with its ends attached.
func (ctx *Context) materializeMember(sym *symbols.Symbol, id int64, owner *Instance, feature string) (*Instance, error) {
	if owner == nil && ctx.model.semantics.IsConnectorObjectUsage(sym) {
		var inst *Instance
		err := ctx.materializeConnectorStarting(nil, sym, sym, id, false, func(conn *Instance) { inst = conn })
		return inst, err
	}
	return ctx.materialize(sym, id, owner, feature)
}

// HoldsOnlyContributions reports whether a feature of known multiplicity
// materializes no object of its own: it is abstract, or a scalar whose lower
// bound demands none and whose own body describes none.
func (f *EffectiveFeature) HoldsOnlyContributions() bool {
	mult := f.Multiplicity
	if !mult.Lower.Known || !mult.Upper.Known {
		return false
	}
	return symbols.IsAbstract(f.Symbol) || (!mult.Lower.Infinite && mult.Lower.Value == 0 && f.Scalar())
}

// holdContributions fills an abstract or optional feature with the values the
// features subsetting it contribute, checked against the multiplicity governing it.
func (inst *Instance) holdContributions(ctx *Context, fv *FeatureValue, name string) (*FeatureValue, error) {
	contributed, err := ctx.subsettingContributions(inst, name)
	if err != nil {
		return nil, err
	}
	return inst.holdContributed(ctx, fv, name, contributed)
}

// holdContributed makes fv hold the values the features subsetting it contribute,
// once they conform to its multiplicity and type and are classified as its values.
func (inst *Instance) holdContributed(ctx *Context, fv *FeatureValue, name string, contributed []Value) (*FeatureValue, error) {
	val := sequenceOf(contributed)
	if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
		return nil, err
	}
	val, err := ctx.holdDeclared(inst, fv, val)
	if err != nil {
		return nil, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
	}
	if fv.Feature.Scalar() {
		if len(contributed) == 1 {
			fv.Value = val
		}
	} else {
		fv.Values = val
	}
	fv.Materialized, fv.intrinsic = true, ctx.subsettersDeclared(inst, name)
	fv.Assumed = semantics.IsBehaviorParameter(fv.Feature.Symbol)
	return fv, nil
}

// CompositeTypeOf returns what a feature is materialized from, or nil for one
// that holds a value rather than an object — a default that binds takes precedence
// over instantiation, as in GetFeatureValue above. A usage with features of its own is
// instantiated as itself, so its body governs and an untyped nested part
// materializes at all. Answering costs no allocation, so a caller walking an
// object graph can decide whether to descend before descending.
func (ctx *Context) CompositeTypeOf(feat *EffectiveFeature) *symbols.Symbol {
	if ctx.valueBinds(feat) {
		return nil
	}
	// A variation is materialized from the variant it is bound to, never from
	// itself: it is an abstract classifier of its variants.
	if ctx.model.semantics.IsVariationFeature(feat.Symbol) {
		return nil
	}
	// A reference usage — a subject (SysML.xtext SubjectUsage), a `ref` usage or
	// one that `references` another — holds what is bound to it, never an object of its own.
	if isSubjectUsage(feat.Symbol) || isReferenceUsage(feat.Symbol) {
		return nil
	}
	// A behavior's parameter is referential too (SysML v2 `validateUsageIsReferential`): an
	// object flows into it, so it holds none of its own; a value-typed one keeps its placeholder.
	if semantics.IsBehaviorParameter(feat.Symbol) && !ctx.model.semantics.IsDataType(feat.Type) {
		return nil
	}
	if feat.Symbol != nil && (ctx.declaresFeatures(feat) || untypedOccurrenceUsage(feat) || satisfyAssertionUsage(feat.Symbol)) {
		return feat.Symbol
	}
	return feat.Type
}

// declaresFeatures reports whether a usage's body, or that of a feature it redefines or
// directly subsets, declares features: a specializing feature inherits them (KerML 1.0 §7.3.4.5).
// A satisfy usage binds its subject (`by`, or the owner without one) as a body member would.
func (ctx *Context) declaresFeatures(feat *EffectiveFeature) bool {
	if declaresFeatures(feat.Symbol) {
		return true
	}
	for _, redefined := range ctx.redefinedFeatures(feat.Symbol, feat.OwnerType) {
		if declaresFeatures(redefined) {
			return true
		}
	}
	for _, sup := range ctx.model.semantics.DeclaredSupertypes(feat.Symbol) {
		if usageSymbol(sup) && declaresFeatures(sup) {
			return true
		}
	}
	return false
}

// untypedOccurrenceUsage reports an occurrence, port, action or state usage declaring no
// type, which its kind's implicit base still classifies (SysML v2 §7.9.2: an untyped `item`
// is an Items::Item, an untyped `port` a Ports::Port; §7.16.2, §7.17.2: an untyped `action`
// an Actions::Action, an untyped `state` a States::StateAction).
func untypedOccurrenceUsage(feat *EffectiveFeature) bool {
	if feat.Type != nil || !(isOccurrenceUsage(feat.Symbol) || untypedPortUsage(feat.Symbol) || untypedPerformanceUsage(feat.Symbol)) {
		return false
	}
	usage := feat.Symbol.Decl.(*ast.Usage)
	return !usage.IsReference
}

// untypedPerformanceUsage reports an action or state usage declaring no type and no value,
// a performance its owner holds as it holds a typed one (Parts.sysml `ownedActions`,
// `ownedStates`; Actions.sysml `subactions`): one object for a usage declaring no
// multiplicity, as many as a fixed bound states, none made up for an open bound.
func untypedPerformanceUsage(sym *symbols.Symbol) bool {
	if sym == nil || (sym.Kind != symbols.SymbolActionUsage && sym.Kind != symbols.SymbolStateUsage) {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Value == nil
}

// untypedPortUsage reports a port usage, whose object a binding connector can join to another's.
func untypedPortUsage(sym *symbols.Symbol) bool {
	if sym == nil || sym.Kind != symbols.SymbolPortUsage {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Value == nil
}

// isSubjectUsage reports whether sym is the subject parameter of a case.
func isSubjectUsage(sym *symbols.Symbol) bool {
	return semantics.IsSubjectUsage(sym)
}

// isReferenceUsage reports a usage declared `ref` or with a `references` relationship
// (SysML v2 §7.6.2: Usage::isReference), which owns none of the objects it holds.
func isReferenceUsage(sym *symbols.Symbol) bool {
	return semantics.IsReferenceUsage(sym)
}

// declaresFeatures reports whether a usage's own body restates or adds features,
// which the object it materializes has to carry.
func declaresFeatures(sym *symbols.Symbol) bool {
	return declaresAFeature(unwrappedDeclMembers(sym.Decl))
}

// declaresAFeature reports whether one of a body's members declares a feature: a
// named or specializing usage, or a state body's named substate or entry, do or
// exit action, each a feature of the state declaring it (SysML v2 §7.18.2).
func declaresAFeature(members []ast.Node) bool {
	for _, member := range members {
		switch m := member.(type) {
		case *ast.Usage:
			if m.Ident.Name != "" || m.Ident.ShortName != "" || len(m.Relationships) > 0 {
				return true
			}
		case *ast.SubjectMember:
			// A subject the body values or defaults (`subject = x;`) restates the inherited parameter.
			if m.BindingExpr != nil {
				return true
			}
		case *ast.SubstateMember:
			if m.Name != "" {
				return true
			}
		case *ast.StateNode:
			if m.Name != "" {
				return true
			}
		case *ast.EntryMember:
			if declaresAFeature(m.Actions) {
				return true
			}
		case *ast.DoMember:
			if declaresAFeature(m.Actions) {
				return true
			}
		case *ast.ExitMember:
			if declaresAFeature(m.Actions) {
				return true
			}
		}
	}
	return false
}

// valueBinds reports whether the value bound to a feature governs it: a redefining
// declaration's own value body governs over one the redefined declaration wrote,
// being the more specific declaration of the feature (KerML 1.0 §7.3.4.5).
func (ctx *Context) valueBinds(feat *EffectiveFeature) bool {
	if feat == nil || feat.DefaultValue == nil {
		return false
	}
	return !ctx.bodyGovernsInheritedValue(feat)
}

// bodyGovernsInheritedValue reports whether a feature's own body values what the value
// it inherits from the declaration it redefines would supply, superseding that value.
func (ctx *Context) bodyGovernsInheritedValue(feat *EffectiveFeature) bool {
	if feat.GovernedByChain {
		return true
	}
	if feat.Symbol == nil || feat.DefaultDecl == nil || feat.DefaultDecl == feat.Symbol {
		return false
	}
	return ctx.restatedValueInBody(feat.Symbol, feat.Type) != ""
}

// restatedInValuedBody returns the name of a feature valued again in the body of the
// declaration that binds a value to it — two values, neither more specific — or ""
// when there is none. A body over an inherited value governs instead, above.
func (ctx *Context) restatedInValuedBody(feat *EffectiveFeature) string {
	if feat.Symbol == nil {
		return ""
	}
	decl, ok := feat.Symbol.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		return ""
	}
	return ctx.restatedValueInBody(feat.Symbol, feat.Type)
}

// restatedByNestedChain returns the next segment of a nested redefinition a
// type of inst applies below the value-bound feature name — a bound value
// supplies the feature's own features, so a chain redefining one below it
// states two values as a restating body would — or "" when none does.
func (ctx *Context) restatedByNestedChain(inst *Instance, name string, feat *EffectiveFeature) string {
	if feat.Symbol == nil {
		return ""
	}
	decl, ok := feat.Symbol.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		return ""
	}
	for _, p := range ctx.pendingNestedRedefinitions(inst, name) {
		if len(p.rest) > 0 && valuedChain(p) && !ctx.chainGovernsValue(p.sym, feat.Symbol) {
			return p.rest[0]
		}
	}
	return ""
}

// restatedValueInBody returns the name of a feature the body of sym values
// again — restating it with `:>>`/`:>`, or re-declaring a feature typ carries —
// or "" when the body values none of them.
func (ctx *Context) restatedValueInBody(sym, typ *symbols.Symbol) string {
	inherited := make(map[string]bool)
	for _, f := range ctx.FeaturesOf(typ) {
		inherited[f.Name] = true
	}
	for _, member := range unwrappedDeclMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || !valuesAFeature(usage) {
			continue
		}
		if name := restatedFeatureName(usage); name != "" {
			return name
		}
		if inherited[usage.Ident.Name] {
			return usage.Ident.Name
		}
	}
	return ""
}

// bodyBindsAFeature reports whether feat's declaration, or one it redefines, binds a
// feature of what it holds (`:>> unitConversion { :>> prefix = kilo; }`): one object, not none.
func (ctx *Context) bodyBindsAFeature(feat *EffectiveFeature) bool {
	if feat.Symbol == nil || symbols.IsAbstract(feat.Symbol) {
		return false
	}
	if ctx.bindsAFeature(feat.Symbol) {
		return true
	}
	for _, redefined := range ctx.redefinedFeatures(feat.Symbol, feat.OwnerType) {
		if ctx.bindsAFeature(redefined) {
			return true
		}
	}
	return false
}

// bindsAFeature reports whether a declaration's body binds a value to a feature of its object,
// directly or under a nested one that exists whenever it does (not an optional or abstract one).
// A framing member (`transformation { :>> target = that; }`) binds no field of it.
func (ctx *Context) bindsAFeature(sym *symbols.Symbol) bool {
	if sym == nil || sym.Scope == nil {
		return false
	}
	for _, member := range sym.Scope.AllMembers() {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !holdsRecordField(member) || ctx.model.semantics.FrameFeature(member) {
			continue
		}
		if usage.Value != nil {
			return true
		}
		if !symbols.IsAbstract(member) && !ctx.optionalValueless(member) && ctx.bindsAFeature(member) {
			return true
		}
	}
	return false
}

// holdsRecordField reports a member whose value is its object's own: a structural shape
// feature, not a behavior, a constraint or a parameter, whose values bind no field.
func holdsRecordField(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolActionUsage, symbols.SymbolStateUsage,
		symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage:
		return false
	}
	return semantics.IsShapeFeature(sym) && !semantics.IsParameter(sym)
}

// valuesAFeature reports whether a usage states a value: its own, or one its
// body states at any depth. A body that only re-declares features states none.
func valuesAFeature(usage *ast.Usage) bool {
	if usage.Value != nil {
		return true
	}
	for _, member := range unwrappedDeclMembers(usage) {
		if nested, ok := member.(*ast.Usage); ok && valuesAFeature(nested) {
			return true
		}
	}
	return false
}

// restatedFeatureName returns the name a usage restates with `:>>` or `:>`, or
// "" when it restates nothing.
func restatedFeatureName(usage *ast.Usage) string {
	for _, rel := range usage.Relationships {
		if rel == nil || (rel.Kind != ast.RelRedefines && rel.Kind != ast.RelSubsets) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok && len(qn.Parts) > 0 {
			return qn.Parts[len(qn.Parts)-1].Text
		}
	}
	return ""
}

// evalFeatureValueDefault evaluates a feature value's default-value expression bound to the
// owning instance. Recursion back through the feature value being computed is reported
// as ErrCyclicFeatureValue rather than recursing until the step budget runs out.
func (ctx *Context) evalFeatureValueDefault(inst *Instance, fv *FeatureValue, name string) (Value, error) {
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.derivingFeatureValues[key] {
		return Value{}, fmt.Errorf("%w: %s.%s", ErrCyclicFeatureValue, inst.Type.Name, name)
	}
	ctx.derivingFeatureValues[key] = true
	defer delete(ctx.derivingFeatureValues, key)
	if fv.body != nil {
		value, ok, err := ctx.readBodyCell(fv.body.owner, fv.body.name)
		if err != nil {
			return Value{}, err
		}
		if !ok {
			return Value{}, &NoValueError{Feature: name}
		}
		return value, nil
	}

	// A declared redefinition of `this` holds the context occurrence the member read answers.
	if r := ctx.model.resolver; r != nil && r.IsOccurrenceThis(fv.Feature.Symbol) {
		return ctx.objectValue(ctx.contextOccurrence(inst))
	}
	scope := fv.Feature.DefaultScope()
	if scope == nil {
		scope = inst.Type.OwnerScope
	}
	ec := NewEvalContextIn(ctx, scope, ctx.bindingContext(inst, fv.Feature))
	if isBehaviorDefKind(inst.Type.Kind) {
		// A feature default of a behavior def is evaluated on the def's
		// occurrence, which the instance being evaluated is.
		ec.occurrence = inst
	}
	defer ec.beginStep()()
	if val, ok, err := ctx.subjectMemberValue(fv.Feature, ec.self, scope, fv.Feature.DefaultValue); ok || err != nil {
		if err != nil {
			return Value{}, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
		}
		return val, nil
	}
	val, err := ec.Eval(fv.Feature.DefaultValue)
	if err != nil {
		return Value{}, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
	}
	return val, nil
}

// subjectMemberValue reads a subject's value expression that is a qualified feature name of
// self's type (`subject subj default Case::result;` on a case object) as self's own member.
func (ctx *Context) subjectMemberValue(feat *EffectiveFeature, self *Instance, scope *symbols.Scope, expr ast.Node) (Value, bool, error) {
	if feat == nil || !isSubjectUsage(feat.Symbol) {
		return Value{}, false, nil
	}
	return ctx.qualifiedMemberOf(self, scope, expr)
}

// qualifiedMemberOf reads a qualified feature name of self's type as self's own member.
func (ctx *Context) qualifiedMemberOf(self *Instance, scope *symbols.Scope, expr ast.Node) (Value, bool, error) {
	qn := ast.AsQualifiedName(expr)
	if self == nil || qn == nil || len(qn.Parts) < 2 {
		return Value{}, false, nil
	}
	feature, ok := ctx.readQualified(scope, qn).Symbol()
	if !ok || feature == nil || !ctx.model.semantics.Conforms(self.Type, ctx.findOwnerType(feature)) {
		return Value{}, false, nil
	}
	fv, err := self.MemberValue(ctx, feature.Name)
	if err != nil {
		return Value{}, true, err
	}
	return fv.HeldValue(), true, nil
}
