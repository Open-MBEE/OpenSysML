package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxMaterializedLowerBound bounds the anonymous objects a collection feature value is
// filled with: a lower bound past it is a model that cannot be materialized
// rather than a run that is merely slow.
const maxMaterializedLowerBound int64 = 1000

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
	Value          Value // scalar feature value (multiplicity [1])
	Values         Value // collection feature value (Sequence or Set)
	Materialized   bool  // lazy flag: has this feature value been instantiated?
	Written        bool  // a run assigned this value, so no default derives it again
	BindingDerived bool  // value came from binding propagation rather than a write
	// dependents are the derived values that read this one, to unmaterialize when
	// it changes; nil until one does (see dependents.go).
	dependents []*FeatureValue
	// reads are the feature values a `=` value listed itself on, to delist from when
	// derived again; nil until it is derived.
	reads []*FeatureValue
	// changing is set while a write to this value is under way, so a write nested
	// in it counts as part of it (see beforeWrite).
	changing bool
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
		return collectionOf(s.Feature, nil), nil
	}
	return Value{}, fmt.Errorf("%w: %s", ErrUninitializedFeatureValue, name)
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
	for _, feat := range features {
		if !ctx.libraryDeclared(feat.Symbol) && holdsRecordField(feat.Symbol) {
			return true
		}
	}
	return false
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
	mark := len(ctx.created)
	inst, err := ctx.materialize(sym, 0, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	// Registered before its behaviors start, so one of them naming the usage
	// reaches this object; a failed start abandons the occurrence with it.
	inst.explicit = true
	prior, hadPrior := ctx.occurrences[sym]
	if ctx.registersOccurrence(sym) {
		ctx.occurrences[sym] = inst.ID
	}
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		if hadPrior {
			ctx.occurrences[sym] = prior
		}
		return nil, err
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
		ctx.restatedInValuedBody(feat) == "" && !ctx.defaultYieldsToSubsetters(inst, feat) {
		if semVal, ok := ctx.model.semantics.Eval(feat.DefaultValue); ok {
			val := Value{Kind: ValConst, Const: semVal}
			if ctx.checkDefault(inst, fv, feat.Name, &val, admitDeclared) == nil {
				fv.Value = val
				fv.Materialized = true
			}
		}
	}
}

// defaultYieldsToSubsetters reports whether feat's `default` may be superseded by a
// feature of one of inst's types subsetting it, so it must wait to be read rather than be folded.
func (ctx *Context) defaultYieldsToSubsetters(inst *Instance, feat *EffectiveFeature) bool {
	if !feat.DefaultIsFallback() {
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
		for _, name := range ctx.subsettedNames(features[i].Symbol, typ) {
			fv, ok := inst.FeatureValues[name]
			if !ok || !fv.Materialized || fv.Written || !ctx.valueBinds(fv.Feature) || !fv.Feature.DefaultIsFallback() {
				continue
			}
			ctx.noteProbeWrite(fv)
			fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
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

	if _, taken := ctx.instances[id]; taken || id <= 0 {
		id = ctx.allocateID()
	}
	ctx.ids.atLeast(id + 1)

	// Get effective features
	features := ctx.FeaturesOf(sym)

	// Create instance
	inst := &Instance{
		ID:            id,
		Type:          sym,
		FeatureValues: make(map[string]*FeatureValue, len(features)),
		owner:         owner,
		ownerFeature:  feature,
	}

	// Create feature value for each feature, allocated as one block.
	values := make([]FeatureValue, len(features))
	for i := range features {
		fv := &values[i]
		ctx.initFeatureValue(inst, fv, &features[i])
		inst.FeatureValues[features[i].Name] = fv
	}

	// A redefining feature declares the feature it redefines again, so the two
	// names read one feature value.
	if err := ctx.aliasRedefinedFeatureValuesOf(inst, sym, nil); err != nil {
		return nil, err
	}

	// Register instance
	ctx.registerInstance(inst)
	ctx.beginLife(inst)

	if ctx.trace != nil {
		ctx.trace.RecordObjectMaterialized(symbolText(sym), inst.ID)
	}

	return inst, nil
}

// occurrenceOf returns the object a usage denotes, materializing it once: a part
// declared in a package names one occurrence, so reading its features twice
// reads the same object.
func (ctx *Context) occurrenceOf(sym *symbols.Symbol) (*Instance, error) {
	if id, ok := ctx.occurrences[sym]; ok {
		if inst, ok := ctx.instances[id]; ok {
			return inst, nil
		}
	}
	// The occurrence is recorded before its behaviors start, so a behavior that
	// reaches the usage it belongs to reads this object rather than a second one.
	mark := len(ctx.created)
	inst, err := ctx.materialize(sym, 0, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	ctx.occurrences[sym] = inst.ID
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
	return inst, nil
}

// OccurrenceUsage is the qualified name of the declared usage inst is the
// occurrence of, or "" for an object materialized any other way.
func (ctx *Context) OccurrenceUsage(inst *Instance) string {
	if inst == nil {
		return ""
	}
	if id, ok := ctx.occurrences[inst.Type]; !ok || id != inst.ID {
		return ""
	}
	return ctx.qualifiedSymbolName(inst.Type)
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
	switch usage.Kind {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence, ast.UsageIndividual:
		return true
	default:
		return false
	}
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

// occursOnce reports whether a usage names at most one occurrence; several
// occurrences are a collection rather than one object to read features from.
func (ctx *Context) occursOnce(sym *symbols.Symbol) bool {
	mult := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym))
	return !mult.Upper.Infinite && mult.Upper.Value <= 1
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
	lower := ctx.featureMultiplicity(sym, ctx.findOwnerType(sym)).Lower
	return lower.Known && !lower.Infinite && lower.Value == 0
}

// checkDefault reports a value the feature does not admit: a count outside its
// multiplicity (1..1 when none is declared) or an element outside its type.
func (ctx *Context) checkDefault(inst *Instance, fv *FeatureValue, name string, val *Value, how admission) error {
	return ctx.checkAdmits(fv.Feature, fmt.Sprintf("feature value %s.%s", inst.Type.Name, name), val, how)
}

// checkAdmits reports a value the feature does not admit, by count or by type,
// naming the value as what.
func (ctx *Context) checkAdmits(feat *EffectiveFeature, what string, val *Value, how admission) error {
	if msg := feat.Multiplicity.CountViolation(elementCount(val)); msg != "" {
		return fmt.Errorf("%s: %w: %s", what, ErrMultiplicityViolation, msg)
	}
	return ctx.checkWriteType(feat.DeclScope(), what, feat.Type, val, how)
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
	if !feat.Scalar() && (val.Kind != ValSequence && val.Kind != ValSet || feat.HoldsSet != (val.Kind == ValSet)) {
		// A multi-valued feature holds a collection however it was written, so a
		// single value written to one is that collection's one element.
		elements := elementsOf(val)
		if err := ctx.chargeElements(int64(len(elements))); err != nil {
			return Value{}, err
		}
		val = collectionOf(feat, elements)
	} else if feat.Scalar() && (val.Kind == ValSequence || val.Kind == ValSet) {
		// A scalar feature holds the one element of a one-element collection.
		if elements := elementsOf(val); len(elements) == 1 {
			val = elements[0]
		}
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
	if err := ctx.checkNotDestroyed(inst); err != nil {
		return nil, err
	}
	if _, ok := inst.FeatureValues[name]; !ok {
		// Naming no feature value of the object is no materialization of one.
		return nil, fmt.Errorf("%w: feature %q not found in instance %d (type %s)", ErrNoSuchFeature, name, inst.ID, inst.Type.Name)
	}
	fv, err := inst.materializeFeatureValue(ctx, name)
	if err != nil {
		return nil, &FeatureValueError{Err: err}
	}
	return fv, nil
}

// SetFeatureValue writes a value to the named feature value of the object, which is how a
// behavior the object performs updates the object's own state. The value must
// conform to the multiplicity governing the feature; a feature the object does
// not have is reported rather than added.
func (inst *Instance) SetFeatureValue(ctx *Context, name string, value Value) error {
	if err := ctx.checkNotDestroyed(inst); err != nil {
		return err
	}
	fv, ok := inst.FeatureValues[name]
	if !ok {
		return fmt.Errorf("%w: feature %q not found in instance %d (type %s)", ErrNoSuchFeature, name, inst.ID, inst.Type.Name)
	}
	// Checked before the write, so a value the feature does not admit leaves it
	// holding what it held.
	if err := ctx.checkDefault(inst, fv, name, &value, admitWritten); err != nil {
		return err
	}
	value, err := ctx.admitted(fv.Feature, value, admitWritten)
	if err != nil {
		return err
	}
	ctx.noteProbeWrite(fv)
	before := ctx.beforeWrite(fv)
	if fv.Feature.Scalar() {
		fv.Value = value
		fv.Values = Value{}
	} else {
		fv.Values = value
		fv.Value = Value{}
	}
	fv.Materialized, fv.Written = true, true
	fv.BindingDerived = false
	ctx.afterWrite(fv, before)
	return nil
}

// materializeFeatureValue is GetFeatureValue's materialization: the feature value's value, evaluated and
// checked against the multiplicity governing its feature the first time it is read.
func (inst *Instance) materializeFeatureValue(ctx *Context, name string) (*FeatureValue, error) {
	defer ctx.beginRun()()

	fv := inst.FeatureValues[name]
	before := ctx.beforeWrite(fv)
	err := inst.materializeBoundOrIntrinsic(ctx, fv, name)
	ctx.afterWrite(fv, before)
	if err != nil {
		return nil, err
	}
	ctx.noteRead(fv)
	return fv, nil
}

// materializeBoundOrIntrinsic gives fv the value a binding determines, else the one
// its feature states, once.
func (inst *Instance) materializeBoundOrIntrinsic(ctx *Context, fv *FeatureValue, name string) error {
	if val, found, err := ctx.resolveBindingValue(inst, name); err != nil {
		return err
	} else if found {
		return ctx.assignBindingValue(inst, fv, name, val)
	} else if !fv.Materialized {
		_, err := inst.materializeFeatureValueIntrinsic(ctx, name)
		return err
	}
	return nil
}

// materializeFeatureValueIntrinsic evaluates a feature without following
// binding connectors; binding resolution calls it to inspect an endpoint.
func (inst *Instance) materializeFeatureValueIntrinsic(ctx *Context, name string) (*FeatureValue, error) {
	fv := inst.FeatureValues[name]
	before := ctx.beforeWrite(fv)
	_, err := inst.materializeIntrinsic(ctx, fv, name)
	ctx.afterWrite(fv, before)
	if err != nil {
		return nil, err
	}
	return fv, nil
}

// materializeIntrinsic evaluates fv from what the model states of its feature: a
// variation, a default, the members subsetting it, or the objects it holds.
func (inst *Instance) materializeIntrinsic(ctx *Context, fv *FeatureValue, name string) (*FeatureValue, error) {
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
		fv.Materialized = true
		return fv, nil
	}

	// A bound value supplies the feature's own features, so a body restating one
	// of them states two values for it.
	if restated := ctx.restatedInValuedBody(fv.Feature); restated != "" {
		return nil, fmt.Errorf("feature value %s.%s: %w: %s", inst.Type.Name, name, ErrValuedFeatureRestated, restated)
	}

	// A `default` applies only where nothing else populates the feature: the
	// members subsetting it do (KerML 1.0 §7.3.4.5).
	if ctx.valueBinds(fv.Feature) && fv.Feature.DefaultIsFallback() {
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
		val, err := ctx.deriveFeatureValue(inst, fv, name)
		if err != nil {
			return nil, err
		}
		if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
			return nil, err
		}
		if val, err = ctx.admitted(fv.Feature, val, admitDeclared); err != nil {
			return nil, err
		}
		ctx.noteProbeWrite(fv)
		if fv.Feature.Scalar() {
			fv.Value = val
		} else {
			fv.Values = val
		}
		fv.Materialized = true
		return fv, nil
	}

	// A collection an optional feature subsets fills its objects: it is read first.
	if err := ctx.materializeSubsettedCollections(inst, fv); err != nil {
		return nil, err
	}
	if fv.Materialized {
		return fv, nil
	}

	// An abstract feature has no values of its own (KerML 1.0 §7.3.3.1) and an
	// optional one demands none: each, a connector included, holds only contributions —
	// unless the declaration's body binds a feature of the one object it then holds.
	if fv.Feature.HoldsOnlyContributions() && !ctx.bodyBindsAFeature(fv.Feature) && (ctx.model.semantics.IsConnectorUsage(fv.Feature.Symbol) || ctx.CompositeTypeOf(fv.Feature) != nil) {
		return inst.holdContributions(ctx, fv, name)
	}

	// A connector holds the features it connects at its ends rather than objects
	// of its own, so it is materialized from what the `connect` clause names.
	if ctx.model.semantics.IsConnectorUsage(fv.Feature.Symbol) {
		if err := ctx.materializeConnectorFeatureValue(inst, fv, name); err != nil {
			return nil, err
		}
		return fv, nil
	}

	// Lazy instantiation: a composite feature holds objects of its own.
	if composite := ctx.CompositeTypeOf(fv.Feature); composite != nil {
		// Check multiplicity (C2 + C1)
		mult := fv.Feature.Multiplicity
		if !mult.Upper.Known || !mult.Lower.Known {
			return nil, fmt.Errorf("cannot materialize feature %q with unknown multiplicity", name)
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
			fv.Materialized = true
			if err := ctx.startClassifierBehaviors(childInst, mark); err != nil {
				return nil, err
			}
		} else {
			// Guard against infinite/huge lower bound (C3)
			if mult.Lower.Infinite || mult.Lower.Value > maxMaterializedLowerBound {
				return nil, fmt.Errorf("%w: lower bound too large or infinite for feature %q", ErrMultiplicityViolation, name)
			}

			// Subsetting features' objects are members; optional subsetters with room, then
			// anonymous objects, make up the lower bound. A failed read leaves nothing behind.
			release := ctx.elementScope()
			contributed, err := ctx.subsettingContributions(inst, name)
			if err != nil {
				release()
				return nil, err
			}

			count := int(mult.Lower.Value) - len(contributed)
			if count < 0 {
				count = 0
			}

			// The whole collection is held before any of its objects starts, so a
			// behavior reading the feature back reads the objects held in it.
			if err := ctx.chargeElements(int64(count)); err != nil {
				release()
				return nil, err
			}
			mark := len(ctx.created)
			children, unfill, err := ctx.fillOptionalSubsetters(inst, name, count)
			fail := func(err error) (*FeatureValue, error) {
				fv.Values, fv.Materialized = Value{}, false
				ctx.abandonInstancesSince(mark)
				unfill()
				release()
				return nil, err
			}
			if err != nil {
				return fail(err)
			}
			seq := NewSequence()
			for _, val := range contributed {
				seq.Append(val)
			}
			for _, child := range children {
				seq.Append(Value{Kind: ValInstance, Instance: child.ID})
			}
			for i := len(children); i < count; i++ {
				childInst, err := ctx.materialize(composite, 0, inst, name)
				if err != nil {
					return fail(err)
				}
				seq.Append(Value{Kind: ValInstance, Instance: childInst.ID})
				children = append(children, childInst)
			}
			fv.Values = collectionOf(fv.Feature, seq.Elements())
			fv.Materialized = true
			if err := ctx.startClassifierBehaviorsOf(children, mark); err != nil {
				return fail(err)
			}
		}
		fv.Materialized = true
	}

	return fv, nil
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
	val, err := ctx.admitted(fv.Feature, val, admitDeclared)
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
	fv.Materialized = true
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
	// A subject is a reference usage (SysML.xtext SubjectUsage): it holds what
	// is bound to it, never an object of its own.
	if isSubjectUsage(feat.Symbol) {
		return nil
	}
	if feat.Symbol != nil && (ctx.declaresFeatures(feat) || untypedOccurrenceUsage(feat)) {
		return feat.Symbol
	}
	return feat.Type
}

// declaresFeatures reports whether a usage's body, or that of a feature it redefines, declares
// features: a redefining feature inherits the redefined one's (KerML 1.0 §7.3.4.5).
func (ctx *Context) declaresFeatures(feat *EffectiveFeature) bool {
	if declaresFeatures(feat.Symbol) {
		return true
	}
	for _, redefined := range ctx.redefinedFeatures(feat.Symbol, feat.OwnerType) {
		if declaresFeatures(redefined) {
			return true
		}
	}
	return false
}

// untypedOccurrenceUsage reports an occurrence usage declaring no type, which its kind's implicit
// base still classifies (SysML v2 §7.9.2: an untyped `item` is an Items::Item).
func untypedOccurrenceUsage(feat *EffectiveFeature) bool {
	if feat.Type != nil || !isOccurrenceUsage(feat.Symbol) {
		return false
	}
	usage := feat.Symbol.Decl.(*ast.Usage)
	return !usage.IsReference
}

// isSubjectUsage reports whether sym is the subject parameter of a case.
func isSubjectUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch decl := sym.Decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.Usage:
		return decl.Kind == ast.UsageSubject
	}
	return false
}

// declaresFeatures reports whether a usage's own body restates or adds features,
// which the object it materializes has to carry.
func declaresFeatures(sym *symbols.Symbol) bool {
	for _, member := range declMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		if usage.Ident.Name != "" || usage.Ident.ShortName != "" || len(usage.Relationships) > 0 {
			return true
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

// restatedValueInBody returns the name of a feature the body of sym values
// again — restating it with `:>>`/`:>`, or re-declaring a feature typ carries —
// or "" when the body values none of them.
func (ctx *Context) restatedValueInBody(sym, typ *symbols.Symbol) string {
	inherited := make(map[string]bool)
	for _, f := range ctx.FeaturesOf(typ) {
		inherited[f.Name] = true
	}
	for _, member := range declMembers(sym.Decl) {
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
	for _, member := range declMembers(usage) {
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

	scope := fv.Feature.DefaultScope()
	if scope == nil {
		scope = inst.Type.OwnerScope
	}
	ec := NewEvalContextIn(ctx, scope, inst)
	defer ec.beginStep()()
	val, err := ec.Eval(fv.Feature.DefaultValue)
	if err != nil {
		return Value{}, fmt.Errorf("feature value %s.%s: %w", inst.Type.Name, name, err)
	}
	return val, nil
}
