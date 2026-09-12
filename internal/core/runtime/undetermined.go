package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// UndeterminedText is how every surface spells a result the model does not
// determine: a model-level read of a valueless or open-count feature, or over one.
const UndeterminedText = "<undetermined>"

// Undetermined is the payload of a ValUndetermined value: what the model does fix
// about a result it leaves open — the count its values conform to, the values it certainly holds.
type Undetermined struct {
	reason  string
	count   semantics.Range
	known   []Value
	feature *symbols.Symbol // the feature read, whose members a chain resolves against
}

// Reason names why the result is undetermined, phrased for a diagnostic.
func (u *Undetermined) Reason() string {
	if u == nil {
		return ""
	}
	return u.reason
}

// Count is the multiplicity the undetermined values conform to.
func (u *Undetermined) Count() semantics.Range {
	if u == nil {
		return openRange()
	}
	return u.count
}

// Known lists the values the result certainly contains.
func (u *Undetermined) Known() []Value {
	if u == nil {
		return nil
	}
	return u.known
}

// NewUndeterminedValue is an undetermined result of count values, for reason.
func NewUndeterminedValue(reason string, count semantics.Range) Value {
	return Value{Kind: ValUndetermined, ref: &Undetermined{reason: reason, count: count}}
}

// undeterminedFeatureValue is the undetermined read of feature sym, of count values.
func undeterminedFeatureValue(reason string, count semantics.Range, sym *symbols.Symbol) Value {
	return Value{Kind: ValUndetermined, ref: &Undetermined{reason: reason, count: count, feature: sym}}
}

// chainThroughUndetermined reads parts from an undetermined feature read: unresolved
// when its type declares no such member, else undetermined over all it may hold.
func (ec *EvalContext) chainThroughUndetermined(value Value, parts []ast.NameSegment, from string) (Value, error) {
	u := value.Undetermined()
	if u == nil || u.feature == nil {
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}
	cur, count := u.feature, u.Count()
	for _, part := range parts {
		next, ok := ec.ctx.declaredMember(cur, part.Text)
		if !ok {
			return Value{}, fmt.Errorf("%w: %s has no member %s", ErrUnresolvedReference, cur.Name, part.Text)
		}
		count = count.Times(ec.ctx.featureMultiplicity(next, ec.ctx.findOwnerType(next)))
		cur = next
	}
	return undeterminedFeatureValue(u.reason, count, cur), nil
}

// Undetermined returns the payload of a ValUndetermined value, nil for any other.
func (v Value) Undetermined() *Undetermined {
	if v.Kind != ValUndetermined {
		return nil
	}
	u, _ := v.ref.(*Undetermined)
	return u
}

// undeterminedIn returns the first undetermined value among vals, ok when there is one.
func undeterminedIn(vals ...Value) (Value, bool) {
	for _, v := range vals {
		if v.Kind == ValUndetermined {
			return v, true
		}
	}
	return Value{}, false
}

// openRange is the multiplicity that fixes nothing, `[0..*]`.
func openRange() semantics.Range {
	return semantics.Range{
		Lower: semantics.Bound{Value: 0, Known: true},
		Upper: semantics.Bound{Infinite: true, Known: true},
	}
}

// optionalRange is the multiplicity of at most one value, `[0..1]`.
func optionalRange() semantics.Range {
	return semantics.Range{
		Lower: semantics.Bound{Value: 0, Known: true},
		Upper: semantics.Bound{Value: 1, Known: true},
	}
}

// undeterminedResult is the result of an operation over an undetermined operand:
// one value, or possibly none when such an operand may itself be empty.
func undeterminedResult(operands ...Value) Value {
	first, _ := undeterminedIn(operands...)
	count := semantics.AssumedRange()
	for _, v := range operands {
		if u := v.Undetermined(); u != nil && u.Count().AllowsNone() {
			count = optionalRange()
		}
	}
	return NewUndeterminedValue(first.Undetermined().Reason(), count)
}

// undeterminedOne is an undetermined result of exactly one value, whatever the
// operands' counts: the answer of a function that always answers, unknown here.
func undeterminedOne(operands ...Value) Value {
	return undeterminedOf(semantics.AssumedRange(), operands...)
}

// undeterminedOf is an undetermined result carrying count, for the reason of the
// first undetermined operand.
func undeterminedOf(count semantics.Range, operands ...Value) Value {
	first, _ := undeterminedIn(operands...)
	return NewUndeterminedValue(first.Undetermined().Reason(), count)
}

// undeterminedElements is a sequence some element of which is undetermined: its
// count adds up the elements' and it certainly holds the determined ones.
func undeterminedElements(elements []Value) Value {
	first, _ := undeterminedIn(elements...)
	count := semantics.CountRange(0)
	var known []Value
	for _, elem := range elements {
		count = count.Plus(countOf(elem))
		known = append(known, knownElementsOf(elem)...)
	}
	return Value{Kind: ValUndetermined, ref: &Undetermined{
		reason: first.Undetermined().Reason(), count: count, known: known,
	}}
}

// undeterminedFiltered is a filter's result when the test is undetermined for
// some elements: it certainly holds kept, and possibly each of those.
func undeterminedFiltered(kept, open []Value) Value {
	count := semantics.Range{
		Lower: semantics.Bound{Value: int64(len(kept)), Known: true},
		Upper: semantics.Bound{Value: int64(len(kept) + len(open)), Known: true},
	}
	return Value{Kind: ValUndetermined, ref: &Undetermined{
		reason: open[0].Undetermined().Reason(), count: count, known: kept,
	}}
}

// undeterminedAware lists the built-ins that decide over an undetermined argument
// themselves, from the count the model gives or another argument.
var undeterminedAware = map[string]bool{
	"ControlFunctions::if":        true,
	"ControlFunctions::??":        true,
	"ControlFunctions::and":       true,
	"ControlFunctions::or":        true,
	"ControlFunctions::implies":   true,
	"SequenceFunctions::size":     true,
	"SequenceFunctions::isEmpty":  true,
	"SequenceFunctions::notEmpty": true,
	"SequenceFunctions::includes": true,
	"SequenceFunctions::excludes": true,
	"SequenceFunctions::#":        true,
	"BaseFunctions::#":            true,
	"BaseFunctions::,":            true,
}

// undeterminedInvocation applies a function that does not decide open arguments
// itself to one: undetermined, of the count its result declares.
func (ctx *Context) undeterminedInvocation(name string, args []Value) (Value, bool) {
	if undeterminedAware[name] {
		return Value{}, false
	}
	if _, open := undeterminedIn(args...); !open {
		return Value{}, false
	}
	return undeterminedOf(ctx.libraryResultCount(name), args...), true
}

// libraryResultCount is the multiplicity the library declares for the result of
// the function fqn; `[0..*]` where the library is not loaded or declares none.
func (ctx *Context) libraryResultCount(fqn string) semantics.Range {
	fn := ctx.librarySymbol(fqn)
	if fn == nil {
		return openRange()
	}
	shape, err := ctx.calcInterfaceOf(fn)
	if err != nil {
		return openRange()
	}
	if result := shape.resultOutput(); result != nil && result.Decl.Target != nil {
		return result.Decl.Target.mult
	}
	return openRange()
}

// countOf is the multiplicity the values of val conform to: exact for a
// determined value, the bounds carried for an undetermined one.
func countOf(val Value) semantics.Range {
	if u := val.Undetermined(); u != nil {
		return u.Count()
	}
	return semantics.CountRange(int64(len(elementsOf(val))))
}

// heldCountOf is countOf without materializing a scalar's one-element sequence.
func heldCountOf(val *Value) semantics.Range {
	if u := val.Undetermined(); u != nil {
		return u.Count()
	}
	return semantics.CountRange(elementCount(val))
}

// declaredCountRefusal says why the bounds of an undetermined value a standalone
// feature declares contradict its multiplicity; a determined value's count is read as declared.
func (ctx *Context) declaredCountRefusal(sym *symbols.Symbol, value *Value) string {
	if value.Undetermined() == nil {
		return ""
	}
	return ctx.featureMultiplicity(sym, nil).HeldViolation(heldCountOf(value))
}

// knownElementsOf lists the elements val certainly holds: all of a determined
// value, those an undetermined one carries.
func knownElementsOf(val Value) []Value {
	if u := val.Undetermined(); u != nil {
		return u.Known()
	}
	return elementsOf(val)
}

// certainlyNonEmpty reports whether the values of val number at least one.
func certainlyNonEmpty(val Value) bool {
	lower := countOf(val).Lower
	return lower.Known && (lower.Infinite || lower.Value >= 1)
}

// certainlyEmpty reports whether the values of val number exactly zero.
func certainlyEmpty(val Value) bool {
	n, ok := countOf(val).Exactly()
	return ok && n == 0
}

// openFeatureRead is the model-level read of an unset or open-count feature of an
// object only the model stands behind: undetermined, of its multiplicity; not ok elsewhere.
func (ec *EvalContext) openFeatureRead(inst *Instance, fv *FeatureValue, from, name string) (Value, bool) {
	feature := fv.Feature
	if !ec.modelLevel() || feature == nil || feature.Symbol == nil {
		return Value{}, false
	}
	if !ec.ctx.readThrough(rootObject(inst)) {
		return Value{}, false
	}
	spelled := name
	if from != "" {
		spelled = from + "." + name
	}
	if ec.ctx.holdsOnlyUnset(fv) {
		return undeterminedFeatureValue(noValueReason(spelled), feature.Multiplicity, feature.Symbol), true
	}
	if !fv.Assumed && (fv.Materialized || fv.HeldValue().Kind != ValInvalid) {
		return Value{}, false
	}
	if _, exact := feature.Multiplicity.Exactly(); exact {
		return Value{}, false
	}
	return undeterminedFeatureValue(openCountReason(spelled, feature.Multiplicity), feature.Multiplicity, feature.Symbol), true
}

// holdsOnlyUnset reports a materialized feature value every element of which is
// unset: an object of a value type standing in for a value nothing gave.
func (ctx *Context) holdsOnlyUnset(fv *FeatureValue) bool {
	held := fv.HeldValue()
	if held.Kind == ValInvalid {
		return false
	}
	elements := elementsOf(held)
	if len(elements) == 0 {
		return false
	}
	for _, elem := range elements {
		if !ctx.HoldsNoValue(elem) {
			return false
		}
	}
	return true
}

// rootObject is the object holding inst, transitively, that no other holds.
func rootObject(inst *Instance) *Instance {
	for inst.owner != nil {
		inst = inst.owner
	}
	return inst
}

// noValueReason phrases a model-level read of a feature nothing gives a value to.
func noValueReason(feature string) string {
	return fmt.Sprintf("%s has no value in the model", feature)
}

// openCountReason phrases a model-level read of a feature whose multiplicity the
// model leaves open.
func openCountReason(feature string, count semantics.Range) string {
	return fmt.Sprintf("%s has multiplicity %s, which fixes no count", feature, count.Text())
}
