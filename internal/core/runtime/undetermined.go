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
	reason    string
	count     semantics.Range
	known     []Value
	positions []Value         // the leading positions the model fixes, see Positions
	feature   *symbols.Symbol // the feature read, whose members a chain resolves against
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

// Positions lists the leading positions the model fixes, in order: a determined value
// fills one, an undetermined one holding an exact count fills that many. Nil fixes none.
func (u *Undetermined) Positions() []Value {
	if u == nil {
		return nil
	}
	return u.positions
}

// positional reports whether Positions are every value u holds.
func (u *Undetermined) positional() bool {
	n, exact := u.Count().Exactly()
	return exact && spanOf(u.Positions()...) == n
}

// spanOf is how many positions entries of Positions fill.
func spanOf(entries ...Value) int64 {
	var n int64
	for _, entry := range entries {
		if u := entry.Undetermined(); u != nil {
			count, _ := u.Count().Exactly()
			n += count
		} else {
			n++
		}
	}
	return n
}

// positionAt is the value at 1-based index among entries of Positions: the determined
// value there, or one unknown value of the open entry; false past them.
func positionAt(entries []Value, index int64) (Value, bool) {
	for _, entry := range entries {
		n := spanOf(entry)
		if index <= n {
			if entry.Kind == ValUndetermined {
				return unknownElementOf(entry), true
			}
			return entry, true
		}
		index -= n
	}
	return Value{}, false
}

// positionsBetween is the entries of Positions from position start through end, an
// open entry cut to the positions of it that lie within.
func positionsBetween(entries []Value, start, end int64) []Value {
	var between []Value
	pos := int64(1)
	for _, entry := range entries {
		n := spanOf(entry)
		lo, hi := max(start, pos), min(end, pos+n-1)
		switch {
		case lo > hi:
		case hi-lo+1 == n:
			between = append(between, entry)
		default:
			u := entry.Undetermined()
			between = append(between, undeterminedFeatureValue(u.Reason(), semantics.CountRange(hi-lo+1), u.feature))
		}
		pos += n
	}
	return between
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
		if u := v.Undetermined(); u != nil && !guaranteesOne(u.Count()) {
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
// count adds up the elements', it certainly holds the determined ones, and it fixes
// the leading positions they do.
func undeterminedElements(elements []Value) Value {
	first, _ := undeterminedIn(elements...)
	count := semantics.CountRange(0)
	var known []Value
	for _, elem := range elements {
		count = count.Plus(countOf(elem))
		known = append(known, knownElementsOf(elem)...)
	}
	return Value{Kind: ValUndetermined, ref: &Undetermined{
		reason: first.Undetermined().Reason(), count: count, known: known, positions: fixedPrefixOf(elements...),
	}}
}

// fixedPrefixOf lists the leading positions vals fix in sequence, as Positions holds
// them: every position of each value up to the first that fixes only some, then those.
func fixedPrefixOf(vals ...Value) []Value {
	var positions []Value
	for _, val := range vals {
		fixed, whole := fixedPositionsOf(val)
		positions = append(positions, fixed...)
		if !whole {
			break
		}
	}
	return positions
}

// fixedPositionsOf lists the leading positions val fixes, as Positions holds them, and
// whether they are every value it holds: all of a determined value, an open value of
// exact count as one entry, else those an open value carries.
func fixedPositionsOf(val Value) ([]Value, bool) {
	u := val.Undetermined()
	if u == nil {
		return elementsOf(val), true
	}
	if u.Positions() == nil && len(u.Known()) == 0 {
		if _, exact := u.Count().Exactly(); exact {
			return []Value{val}, true
		}
	}
	return u.Positions(), u.positional()
}

// sequenceOfPositions is the sequence of entries of Positions: determined where every
// one is, else undetermined fixing the leading positions they do.
func (ec *EvalContext) sequenceOfPositions(entries []Value, sources ...Value) (Value, error) {
	if _, open := undeterminedIn(entries...); open {
		return undeterminedElements(entries), nil
	}
	return ec.sequenceFrom(entries, sources...)
}

// undeterminedFiltered is a filter's result over source when the test is open for
// some elements or source may hold unknown ones it keeps: certainly kept, possibly
// each of the open, and fromUnknown of the unknown.
func undeterminedFiltered(source Value, kept, open []Value, fromUnknown semantics.Range) Value {
	first, _ := undeterminedIn(append([]Value{source}, open...)...)
	count := semantics.CountRange(int64(len(kept))).
		Plus(semantics.Range{Lower: semantics.Bound{Known: true}, Upper: semantics.Bound{Value: int64(len(open)), Known: true}}).
		Plus(fromUnknown)
	return Value{Kind: ValUndetermined, ref: &Undetermined{
		reason: first.Undetermined().Reason(), count: count, known: kept,
	}}
}

// undeterminedCollected is a mapping's result over source when an answer is open or
// source holds unknown elements: the answers' counts plus those mapped from the unknown.
func undeterminedCollected(source Value, answers []Value, fromUnknown semantics.Range) Value {
	first, _ := undeterminedIn(append([]Value{source}, answers...)...)
	count := fromUnknown
	var known []Value
	for _, answer := range answers {
		count = count.Plus(countOf(answer))
		known = append(known, knownElementsOf(answer)...)
	}
	return Value{Kind: ValUndetermined, ref: &Undetermined{
		reason: first.Undetermined().Reason(), count: count, known: known,
	}}
}

// mayHoldUnknown reports whether val may hold a value beyond those it certainly holds.
func mayHoldUnknown(val Value) bool {
	upper := unknownCountOf(val).Upper
	return !upper.Known || upper.Infinite || upper.Value > 0
}

// unknownElementOf is one value of source beyond those it certainly holds, standing
// for any of them: undetermined, a member of the feature source reads where it reads one.
func unknownElementOf(source Value) Value {
	u := source.Undetermined()
	return undeterminedFeatureValue(u.Reason(), semantics.AssumedRange(), u.feature)
}

// unknownCountOf is the count of the values of val beyond those it certainly holds.
func unknownCountOf(val Value) semantics.Range {
	count, held := countOf(val), int64(len(knownElementsOf(val)))
	count.Lower, count.Upper = lessBound(count.Lower, held), lessBound(count.Upper, held)
	return count
}

// lessBound is the finite bound b with n fewer values, at least none.
func lessBound(b semantics.Bound, n int64) semantics.Bound {
	if b.Known && !b.Infinite {
		b.Value = max(b.Value-n, 0)
	}
	return b
}

// undeterminedAware lists the built-ins that decide over an undetermined argument
// themselves, from the count the model gives or another argument; the operator
// forms and the scalar numeric functions register themselves here.
var undeterminedAware = map[string]bool{
	"ControlFunctions::if":        true,
	"ControlFunctions::??":        true,
	"ControlFunctions::and":       true,
	"ControlFunctions::or":        true,
	"ControlFunctions::implies":   true,
	"ControlFunctions::forAll":    true,
	"ControlFunctions::exists":    true,
	"ControlFunctions::allTrue":   true,
	"ControlFunctions::anyTrue":   true,
	"ControlFunctions::select":    true,
	"ControlFunctions::reject":    true,
	"ControlFunctions::selectOne": true,
	"ControlFunctions::collect":   true,
	"SequenceFunctions::size":     true,
	"SequenceFunctions::isEmpty":  true,
	"SequenceFunctions::notEmpty": true,
	"SequenceFunctions::includes": true,
	"SequenceFunctions::excludes": true,
	"SequenceFunctions::#":        true,
	"BaseFunctions::#":            true,
	"BaseFunctions::,":            true,
	// These validate the arguments the model determines before leaving the result open.
	"SequenceFunctions::includingAt": true,
	"SequenceFunctions::subsequence": true,
	"SequenceFunctions::excludingAt": true,
	"StringFunctions::Substring":     true,
	// These answer from the positions an open sequence fixes.
	"SequenceFunctions::head": true,
	"SequenceFunctions::tail": true,
	"SequenceFunctions::last": true,
}

// undeterminedInvocation applies a function that does not decide open arguments
// itself to one: undetermined, of the count its result declares.
func (ctx *Context) undeterminedInvocation(name string, args []Value) (Value, bool) {
	if undeterminedAware[name] {
		return Value{}, false
	}
	return ctx.openInvocation(name, args...)
}

// openInvocation is the result of applying the function name to args when one of them
// is open: undetermined, of the count the library declares for the result.
func (ctx *Context) openInvocation(name string, args ...Value) (Value, bool) {
	if _, open := undeterminedIn(args...); !open {
		return Value{}, false
	}
	return undeterminedOf(ctx.libraryResultCount(name), args...), true
}

// fixedArg reads an argument the model determines through read; an open one is unread.
func fixedArg(val Value, read func(Value) (int64, error)) (n int64, fixed bool, err error) {
	if val.Kind == ValUndetermined {
		return 0, false, nil
	}
	n, err = read(val)
	return n, err == nil, err
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
	return guaranteesOne(countOf(val))
}

// guaranteesOne reports whether count admits no fewer than one value.
func guaranteesOne(count semantics.Range) bool {
	lower := count.Lower
	return lower.Known && (lower.Infinite || lower.Value >= 1)
}

// nonEmptyCount is count restricted to holding at least one value.
func nonEmptyCount(count semantics.Range) semantics.Range {
	if !guaranteesOne(count) {
		count.Lower = semantics.Bound{Value: 1, Known: true}
	}
	return count
}

// certainlyEmpty reports whether the values of val number exactly zero.
func certainlyEmpty(val Value) bool {
	n, ok := countOf(val).Exactly()
	return ok && n == 0
}

// modelReads reports whether reading a feature of inst asserts only what the model
// states: a model-level evaluation reading an object only the model stands behind.
func (ec *EvalContext) modelReads(inst *Instance) bool {
	return ec.modelLevel() && ec.ctx.readThrough(rootObject(inst))
}

// memberFeatureValue reads the named feature value of inst: through the model,
// stopping short of making up a collection's lower bound, or as the object holds it.
func (ec *EvalContext) memberFeatureValue(inst *Instance, name string) (*FeatureValue, *openPopulation, error) {
	if ec.modelReads(inst) {
		return inst.openFeatureValue(ec.ctx, name)
	}
	fv, err := inst.GetFeatureValue(ec.ctx, name)
	return fv, nil, err
}

// openFeatureRead is the model-level read of an unset or open-count feature of an
// object only the model stands behind: undetermined, of its multiplicity; not ok elsewhere.
func (ec *EvalContext) openFeatureRead(inst *Instance, fv *FeatureValue, open *openPopulation, from, name string) (Value, bool) {
	feature := fv.Feature
	if !ec.modelReads(inst) || feature == nil || feature.Symbol == nil {
		return Value{}, false
	}
	spelled := name
	if from != "" {
		spelled = from + "." + name
	}
	if open != nil && open.Stopped {
		return openCollectionValue(spelled, feature, open.Contributed), true
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

// openCollectionValue is the model-level value of a collection whose count the model
// leaves open: undetermined, certainly holding what its subsetters contribute, of a
// count no fewer than they number.
func openCollectionValue(spelled string, feature *EffectiveFeature, contributed []Value) Value {
	mult := feature.Multiplicity
	count := mult
	if held := int64(len(contributed)); held > count.Lower.Value {
		count.Lower = semantics.Bound{Value: held, Known: true}
	}
	return Value{Kind: ValUndetermined, ref: &Undetermined{
		reason: openCountReason(spelled, mult), count: count, known: contributed, feature: feature.Symbol,
	}}
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
