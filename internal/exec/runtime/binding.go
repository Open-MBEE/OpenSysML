package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// bindingConflictError reports unequal values held by the two ends of a
// binding connector.
type bindingConflictError struct {
	Target     string
	Left       string
	Right      string
	LeftValue  Value
	RightValue Value
}

func (e *bindingConflictError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("%s at %s: %s = %s, %s = %s", errBindingConflict,
			e.Target, e.Left, bindingValueText(e.LeftValue), e.Right, bindingValueText(e.RightValue))
	}
	return fmt.Sprintf("%s: %s = %s, %s = %s", errBindingConflict,
		e.Left, bindingValueText(e.LeftValue), e.Right, bindingValueText(e.RightValue))
}

func (e *bindingConflictError) Unwrap() error { return errBindingConflict }

// bindingCycleError reports a binding component that has no valued end.
type bindingCycleError struct {
	Features []string
}

func (e *bindingCycleError) Error() string {
	return fmt.Sprintf("%s: %s", errBindingCycle, strings.Join(e.Features, " -> "))
}

func (e *bindingCycleError) Unwrap() error { return errBindingCycle }

// undeterminedBindingError refuses a namespace binding whose multiplicities make it link
// one unspecified value of an end, which denotes one value only.
type undeterminedBindingError struct {
	Target   string
	Binding  string
	Endpoint string
	// Other is the binding's end opposite Endpoint, as written.
	Other string
}

func (e *undeterminedBindingError) Error() string {
	return fmt.Sprintf("%s: %s is bound by `%s`, which makes some value of %s a value of %s without saying which value of either; the model does not state what %s holds",
		errBindingEnd, e.Target, e.Binding, e.Endpoint, e.Other, e.Endpoint)
}

func (e *undeterminedBindingError) Unwrap() error { return errBindingEnd }

type bindingLocation struct {
	instance *Instance
	name     string
	path     string
}

// bindingEndpoint is one end of a binding: an expression, or the feature its path reaches on every
// object of each collection crossed (KerML 1.0 §7.3.4.6).
type bindingEndpoint struct {
	locations []bindingLocation
	expr      ast.Node
	// index selects one element of what the locations hold, nil for the whole feature;
	// selection is the end as written (`xs#(2)`), for diagnostics.
	index, selection ast.Node
	scope            *symbols.Scope
	// on holds the connector objects an end is read on, one per connector the holder
	// materialized; reached groups each connector's path locations.
	on      []*Instance
	reached [][]bindingLocation
}

// selects reports an end selecting one element of its feature: it reads the element and is
// never assigned through, so the binding constrains the element without determining the feature.
func (e bindingEndpoint) selects() bool { return e.index != nil }

// spread reports a feature end reaching several objects or none, holding their values together.
func (e bindingEndpoint) spread() bool { return e.expr == nil && len(e.locations) != 1 }

// carries reports whether the end reaches the feature value being resolved.
func (e bindingEndpoint) carries(fv *FeatureValue, inst *Instance) bool {
	for _, loc := range e.locations {
		if bindingLocationCarries(loc, fv, inst) {
			return true
		}
	}
	return false
}

// across names the path an end crosses to reach its feature, "" for a feature of the owner.
func (e bindingEndpoint) across() string {
	if len(e.locations) == 0 {
		return ""
	}
	loc := e.locations[0]
	return strings.TrimSuffix(strings.TrimSuffix(loc.path, loc.name), ".")
}

type bindingAttempt struct {
	value       Value
	found       bool
	contributor string
	// settled reports that no end waits on a resolution in progress, so an
	// unfound value means the binding links nothing.
	settled       bool
	cycle         bool
	cycleFeatures []string
	assignment    *bindingAssignment
	err           error
	// defaulted marks the value of a `default =` end, which another binding's value displaces.
	defaulted bool
}

type bindingAssignment struct {
	endpoint bindingEndpoint
	value    Value
	binding  lower.Binding
	end      int
}

func (ctx *Context) objectBindings(typeSym *symbols.Symbol) []lower.Binding {
	if typeSym == nil {
		return nil
	}
	if cached, ok := ctx.model.bindingIR[typeSym]; ok {
		return cached
	}
	var bindings []lower.Binding
	chain := ctx.model.semantics.AllSupertypes(typeSym)
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i] != nil {
			bindings = append(bindings, ctx.declaredBindings(chain[i])...)
			bindings = append(bindings, ctx.connectorOwnedBindings(chain[i])...)
		}
	}
	bindings = append(bindings, ctx.declaredBindings(typeSym)...)
	bindings = append(bindings, ctx.connectorOwnedBindings(typeSym)...)
	ctx.model.bindingIR[typeSym] = bindings
	return bindings
}

// declaredBindings lowers the bindings sym's own declaration states: the binding connectors
// among its members and the feature values its end features' bodies state.
func (ctx *Context) declaredBindings(sym *symbols.Symbol) []lower.Binding {
	scope := DeclScope(sym)
	return append(lower.ToBindings(sym.Decl, scope), lower.EndValueBindings(sym.Decl, scope)...)
}

// connectorOwnedBindings is what the connectors decl declares bind among decl's features: every
// binding each owns (connectorBindings), re-rooted from its ends onto what they attach (ThroughEnds).
func (ctx *Context) connectorOwnedBindings(decl *symbols.Symbol) []lower.Binding {
	if decl == nil || ctx.libraryDeclared(decl) || ctx.model.hoistingBindings[decl] {
		return nil
	}
	ctx.model.hoistingBindings[decl] = true
	defer delete(ctx.model.hoistingBindings, decl)
	var out []lower.Binding
	for _, member := range unwrappedDeclMembers(decl.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || usage.IsVariant {
			continue
		}
		connSym := DeclScope(decl).MemberDeclaring(usage)
		if connSym == nil && usage.Ident.Name == "" && usage.Ident.ShortName == "" {
			connSym = anonymousConnectorSymbol(decl, usage)
		}
		if connSym == nil || !ctx.model.semantics.IsConnectorUsage(connSym) || ctx.connectorMayBeAbsent(connSym, usage) {
			continue
		}
		roots := ctx.connectorEndAttachmentPaths(connSym)
		if len(roots) == 0 {
			continue
		}
		self := lower.Hop{Feature: symbols.LastSegment(connSym.Name)}
		if self.Feature == "" {
			self.Anonymous = usage
		}
		owned := map[string]bool{}
		for _, binding := range ctx.connectorBindings(connSym) {
			ctx.rootConnectorFeatures(roots, owned, connSym, self, binding)
			if through, ok := binding.ThroughEnds(roots, owned, self); ok {
				out = append(out, through)
			}
		}
	}
	return out
}

// connectorBindings is every non-library binding a connector usage owns: its own, its definition's
// and supertypes' declared ones, and those the connectors nested in them bind over their ends.
func (ctx *Context) connectorBindings(connSym *symbols.Symbol) []lower.Binding {
	var out []lower.Binding
	for _, source := range append(ctx.model.semantics.AllSupertypes(connSym), connSym) {
		if source == nil || ctx.libraryDeclared(source) {
			continue
		}
		out = append(out, ctx.declaredBindings(source)...)
		out = append(out, ctx.connectorOwnedBindings(source)...)
	}
	return out
}

// rootConnectorFeatures adds each feature of the connector self a binding end path is rooted at: to roots
// as `self.name`, or to owned when self is anonymous, read on the object its declaration materializes.
func (ctx *Context) rootConnectorFeatures(roots map[string]string, owned map[string]bool,
	connSym *symbols.Symbol, self lower.Hop, binding lower.Binding) {
	for _, end := range binding.Ends {
		head, _, _ := strings.Cut(end.HolderPath(), ".")
		if _, known := roots[head]; known || head == "" || owned[head] {
			continue
		}
		if _, ok := ctx.model.semantics.LookupMember(connSym, head); !ok {
			continue
		}
		if self.Feature != "" {
			roots[head] = self.Feature + "." + head
		} else {
			owned[head] = true
		}
	}
}

// connectorEndAttachmentPaths maps each name of each end of a connector usage —
// the end's own and those of the ends it redefines, by clause or by position in
// a general connector, which are one feature with it — to the feature path the
// end attaches. An end that selects one element of a collection
// (`connect a.ports#(1) to b.q`) attaches a value rather than a feature, so no
// path can be re-rooted at it.
func (ctx *Context) connectorEndAttachmentPaths(connSym *symbols.Symbol) map[string]string {
	ends := ctx.model.semantics.ConnectorEndAttachments(connSym)
	paths := make(map[string]string, len(ends))
	for _, end := range ends {
		if end.Selection != nil {
			continue
		}
		path := lower.FeaturePath(end.Attachment)
		if path == "" {
			continue
		}
		for _, name := range ctx.connectorEndNames(end) {
			if _, taken := paths[name]; !taken {
				paths[name] = path
			}
		}
	}
	return paths
}

// connectorEndNames lists the names under which a connector end's feature is
// known: the effective name of the end and the names of every feature its
// declaration redefines.
func (ctx *Context) connectorEndNames(end semantics.ConnectorEndAttachment) []string {
	var names []string
	if end.Name != "" {
		names = append(names, end.Name)
	}
	for _, sym := range []*symbols.Symbol{end.Declared, end.EndFeature} {
		if sym == nil {
			continue
		}
		if name := symbols.LastSegment(sym.Name); name != "" {
			names = append(names, name)
		}
		for _, redefined := range ctx.model.semantics.AllRedefinedFeatures(sym) {
			if name := symbols.LastSegment(redefined.Name); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// connectorMayBeAbsent reports a connector usage whose stated lower bound is zero:
// `connection c : L [0..*] connect a.p to b.q` may denote no link at all, and a
// link that may not exist binds nothing.
func (ctx *Context) connectorMayBeAbsent(connSym *symbols.Symbol, usage *ast.Usage) bool {
	if usage.Multiplicity == nil {
		return false
	}
	r, ok := ctx.model.semantics.RangeIn(connSym.OwnerScope, usage.Multiplicity)
	return ok && r.Lower.Known && !r.Lower.Infinite && r.Lower.Value == 0
}

func (ctx *Context) bindingsForFeature(typeSym *symbols.Symbol, name string) []lower.Binding {
	if typeSym == nil {
		return nil
	}
	byFeature, ok := ctx.model.bindingFeatures[typeSym]
	if !ok {
		byFeature = make(map[string][]lower.Binding)
		ctx.model.bindingFeatures[typeSym] = byFeature
	}
	if bindings, ok := byFeature[name]; ok {
		return bindings
	}
	var bindings []lower.Binding
	for _, binding := range ctx.objectBindings(typeSym) {
		if bindingInvolvesFeature(binding, name) && !ctx.bindingLinksNothing(binding) {
			bindings = append(bindings, binding)
		}
	}
	byFeature[name] = bindings
	return bindings
}

// bindingLinksNothing reports a binding with an end or connector multiplicity of [0],
// which links no value and so constrains neither feature (KerML 1.0 §7.4.9.2).
// bindingLinks is how many links the binding states it makes, its own lower bound; one, and
// not stated, where it gives none.
func (ctx *Context) bindingLinks(binding lower.Binding) (links int64, stated bool) {
	links = 1
	if r, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Multiplicity); ok && r.Lower.Known && !r.Lower.Infinite {
		links, stated = r.Lower.Value, true
	}
	return links, stated
}

func (ctx *Context) bindingLinksNothing(binding lower.Binding) bool {
	if r, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Multiplicity); ok && r.Upper.Known && !r.Upper.Infinite && r.Upper.Value == 0 {
		return true
	}
	for _, end := range binding.Ends {
		if r, ok := ctx.model.semantics.RangeIn(binding.Scope, end.Multiplicity); ok && r.Upper.Known && !r.Upper.Infinite && r.Upper.Value == 0 {
			return true
		}
	}
	return false
}

func (ctx *Context) resolveBindingValue(inst *Instance, name string) (Value, bool, error) {
	target, ok := inst.FeatureValues[name]
	if !ok {
		return Value{}, false, nil
	}
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.resolvingBindings[key] {
		return Value{}, false, &bindingCycleError{Features: []string{bindingLocationText(bindingLocation{instance: inst, name: name})}}
	}
	if !target.BindingDerived || target.Written || ctx.CompositeTypeOf(target.Feature) != nil {
		return ctx.resolveBindings(inst, target, name, key)
	}
	// A value a binding gave is resolved afresh: clearing it and binding it again is one write.
	before := ctx.beforeWrite(target)
	ctx.noteFeatureWrite(target)
	ctx.noteProbeWrite(target)
	target.Value = Value{}
	target.Values = Value{}
	target.Materialized, target.intrinsic = false, false
	target.BindingDerived, target.Assumed = false, false
	val, found, err := ctx.resolveBindings(inst, target, name, key)
	ctx.afterWrite(target, before)
	return val, found, err
}

// resolveBindings is what the bindings of the target's owners, innermost first, determine it holds.
func (ctx *Context) resolveBindings(inst *Instance, target *FeatureValue, name string, key featureValueRef) (Value, bool, error) {
	var unmet []lower.Binding
	// The innermost `default =` values, applied once no owner's other binding determines the feature.
	var defaults *bindingSetResult
	path := name
	for current := inst; current != nil; {
		var bindings []lower.Binding
		if ctx.declaresBindings(current) {
			bindings = ctx.bindingsOf(current, path)
		}
		if len(bindings) != 0 {
			ctx.resolvingBindings[key] = true
			result, err := ctx.resolveBindingSet(current, inst, target, path, bindings, key, defaults == nil)
			delete(ctx.resolvingBindings, key)
			if err != nil || result.found {
				return result.value, result.found, err
			}
			if defaults == nil && result.defaulting() {
				defaults = &result
			}
			unmet = append(unmet, result.unmet...)
			if len(result.cycleFeatures) != 0 && defaults == nil {
				if err := ctx.unmetBindingCounts(unmet); err != nil {
					return Value{}, false, err
				}
				return Value{}, false, &bindingCycleError{Features: result.cycleFeatures}
			}
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" || !ctx.ownersDeclareBindings(owner) {
			break
		}
		path = ownerFeature + "." + path
		current = owner
	}
	if defaults != nil {
		return ctx.applyDefaults(inst, key, *defaults)
	}
	if err := ctx.unmetBindingCounts(unmet); err != nil {
		return Value{}, false, err
	}
	return Value{}, false, nil
}

// bindingSetResult is what the bindings of one owner determine about a feature.
type bindingSetResult struct {
	// defaultErr is the failure of a `default =` end, reported only when the default applies.
	defaultErr error
	// defaults are the attempts of `default =` ends, when no other binding of the owner states a value.
	defaults      []bindingAttempt
	value         Value
	found         bool
	cycleFeatures []string
	// unmet are the whole bindings that determined nothing, so their ends link no value.
	unmet []lower.Binding
}

// defaulting reports an owner whose bindings determine the feature only by a `default =` value.
func (r bindingSetResult) defaulting() bool {
	return r.defaultErr != nil || len(r.defaults) != 0
}

// unmetBindingCounts refuses a whole binding whose ends link no value while one
// of them requires some: the feature and its other end both hold nothing.
func (ctx *Context) unmetBindingCounts(unmet []lower.Binding) error {
	for _, binding := range unmet {
		if err := ctx.wholeBindingCounts(binding, Value{}); err != nil {
			return err
		}
	}
	return nil
}

// claimBinding marks the binding, by its origin, as the one resolving key, so its own
// probe of the other end is not mistaken for a genuine cycle; the returned func releases the claim.
func (ctx *Context) claimBinding(key featureValueRef, binding program.Origin) func() {
	previousOwner, hadOwner := ctx.bindingOwners[key]
	ctx.bindingOwners[key] = binding
	return func() {
		if hadOwner {
			ctx.bindingOwners[key] = previousOwner
		} else {
			delete(ctx.bindingOwners, key)
		}
	}
}

// attemptBindings tries each binding against the target with its carrying end read as
// carryingRead: through its bindings first, so no object is created while another binding may
// determine the feature; materialized — an object of its own identified with the other end —
// only once none did, at the outermost resolution.
func (ctx *Context) attemptBindings(owner, targetInst *Instance, target *FeatureValue,
	endpointName string, bindings []lower.Binding, key featureValueRef, carryingRead endRead,
) ([]bindingAttempt, bindingSetResult, error) {
	var result bindingSetResult
	var attempts []bindingAttempt
	for _, binding := range bindings {
		partial, err := ctx.partialBinding(owner, targetInst, target, binding, key)
		if err != nil {
			return nil, result, err
		}
		if partial {
			// A binding linking fewer values than an end holds identifies some value of one
			// end with some value of the other: it constrains the ends (checkBindingOn)
			// without determining either, so the feature holds what its own declaration states.
			continue
		}
		attempt := ctx.attemptBinding(owner, targetInst, target, binding, key, carryingRead)
		if attempt.err != nil && attempt.defaulted {
			if result.defaultErr == nil {
				result.defaultErr = attempt.err
			}
			continue
		}
		if attempt.err != nil {
			return nil, result, attempt.err
		}
		if attempt.cycle {
			result.cycleFeatures = attempt.cycleFeatures
		}
		if !attempt.found {
			if attempt.settled {
				result.unmet = append(result.unmet, binding)
			}
			continue
		}
		if err := ctx.wholeBindingCounts(binding, attempt.value); err != nil {
			return nil, result, err
		}
		if end := bindingEndForPath(binding, endpointName); end >= 0 {
			attempt.contributor = ctx.bindingEndpointText(binding, 1-end)
		}
		attempts = append(attempts, attempt)
	}
	return attempts, result, nil
}

// resolveBindingSet is what one owner's bindings determine about the target, `default =` values left in
// result.defaults; materialize lets the carrying end get an object of its own, never while a default is pending.
func (ctx *Context) resolveBindingSet(owner, targetInst *Instance, target *FeatureValue,
	endpointName string, bindings []lower.Binding, key featureValueRef, materialize bool) (bindingSetResult, error) {
	attempts, result, err := ctx.attemptBindings(owner, targetInst, target, endpointName, bindings, key, endBound)
	if err == nil && materialize && len(attempts) == 0 && len(ctx.resolvingBindings) == 1 {
		attempts, result, err = ctx.attemptBindings(owner, targetInst, target, endpointName, bindings, key, endMaterialized)
	}
	if err != nil {
		return result, err
	}
	attempts = withoutDisplacedDefaults(attempts)
	if (len(attempts) == 0 && result.defaultErr != nil) || (len(attempts) != 0 && attempts[0].defaulted) {
		result.defaults = attempts
		return result, nil
	}
	result.value, result.found, err = ctx.settleAttempts(targetInst, key, attempts)
	return result, err
}

// applyDefaults is what the `default =` values of one owner's bindings give the target: the failure
// of one, or the value they agree on.
func (ctx *Context) applyDefaults(targetInst *Instance, key featureValueRef, result bindingSetResult) (Value, bool, error) {
	if result.defaultErr != nil {
		return Value{}, false, result.defaultErr
	}
	ctx.resolvingBindings[key] = true
	defer delete(ctx.resolvingBindings, key)
	return ctx.settleAttempts(targetInst, key, result.defaults)
}

// settleAttempts assigns the target what the found attempts agree on.
func (ctx *Context) settleAttempts(targetInst *Instance, key featureValueRef, attempts []bindingAttempt) (Value, bool, error) {
	if len(attempts) == 0 {
		return Value{}, false, nil
	}
	// Every binding of the whole feature identifies it with its other end, so
	// two of them must agree on its values, a sequence like a scalar.
	for _, attempt := range attempts[1:] {
		if !ctx.equalValues(attempts[0].value, attempt.value) {
			return Value{}, false, &bindingConflictError{
				Target:     bindingLocationText(bindingLocation{instance: targetInst, name: key.feature}),
				Left:       attempts[0].contributor,
				Right:      attempt.contributor,
				LeftValue:  attempts[0].value,
				RightValue: attempt.value,
			}
		}
	}
	if assignment := attempts[0].assignment; assignment != nil {
		if err := ctx.assignBindingEndpoint(
			assignment.endpoint, assignment.value, assignment.binding, assignment.end,
		); err != nil {
			return Value{}, false, err
		}
	}
	return attempts[0].value, true, nil
}

// withoutDisplacedDefaults drops the attempts of `default =` ends once any other binding
// states the feature's value (SysML v2 §7.13.4: a default applies absent other values).
func withoutDisplacedDefaults(attempts []bindingAttempt) []bindingAttempt {
	stated := attempts[:0:0]
	for _, attempt := range attempts {
		if !attempt.defaulted {
			stated = append(stated, attempt)
		}
	}
	if len(stated) == 0 {
		return attempts
	}
	return stated
}

// constrainingBinding reports whether binding constrains its ends on inst without determining
// either: an end reaching several objects, or multiplicities linking fewer values than an end holds.
func (ctx *Context) constrainingBinding(inst *Instance, binding lower.Binding) (bool, error) {
	var endpoints [2]bindingEndpoint
	var unresolved error
	for end := range binding.Ends {
		endpoint, err := ctx.resolveBindingEndpoint(inst, binding, end)
		if err != nil {
			unresolved = errors.Join(unresolved, err)
			continue
		}
		// An end reaching no object links nothing on inst, so the binding states nothing about it.
		if endpoint.expr == nil && len(endpoint.locations) == 0 {
			return false, nil
		}
		endpoints[end] = endpoint
	}
	if unresolved != nil {
		return false, unresolved
	}
	for _, endpoint := range endpoints {
		if endpoint.expr != nil {
			continue
		}
		if endpoint.spread() {
			return true, nil
		}
		loc := endpoint.locations[0]
		target := loc.instance.FeatureValues[loc.name]
		if target == nil {
			continue
		}
		partial, err := ctx.partialBinding(inst, loc.instance, target, binding,
			featureValueRef{instance: loc.instance.ID, feature: loc.name})
		if err != nil || partial {
			return partial, err
		}
	}
	return false, nil
}

// CheckBindingOn checks a binding as the constraint it states about inst (KerML 1.0 §7.4.6, §8.3.4.5.2):
// its ends share as many values as it declares links — equal or sharing one when it declares none.
func (ctx *Context) CheckBindingOn(binding lower.Binding, inst *Instance) (CheckResult, error) {
	defer ctx.beginRun()()
	result := CheckResult{Subject: inst, SubjectRoot: inst}
	var values [2]Value
	for end := range binding.Ends {
		endpoint, err := ctx.resolveBindingEndpoint(inst, binding, end)
		if err != nil {
			return result, err
		}
		val, found, err := ctx.bindingEndpointValue(endpoint, inst, endMaterialized)
		if err != nil {
			return result, err
		}
		if !found {
			return result, fmt.Errorf("%w: %s holds no value", errBindingEnd, ctx.bindingEndpointText(binding, end))
		}
		if val.Kind == ValUndetermined {
			return result, fmt.Errorf("%w: %s is undetermined", errBindingEnd, ctx.bindingEndpointText(binding, end))
		}
		values[end] = val
	}
	links, stated := ctx.bindingLinks(binding)
	if links == 0 || (!stated && ctx.equalValues(values[0], values[1])) {
		result.Holds = true
		return result, nil
	}
	shared := ctx.sharedValues(values[0], values[1])
	if shared >= links {
		result.Holds = true
		return result, nil
	}
	return result, &ViolationError{
		Kind:    "binding",
		Element: ctx.bindingText(binding),
		What:    "identity",
		Condition: fmt.Sprintf("%s and %s share %d value(s) where the binding links %d",
			ctx.bindingEndpointText(binding, 0), ctx.bindingEndpointText(binding, 1), shared, links),
	}
}

// sharedValues counts the values a and b hold in common, a repeated value as often as both hold it.
func (ctx *Context) sharedValues(a, b Value) int64 {
	var shared int64
	others := elementsOf(b)
	used := make([]bool, len(others))
	for _, elem := range elementsOf(a) {
		at := slices.IndexFunc(others, func(o Value) bool { return ctx.equalValues(o, elem) })
		for at >= 0 && used[at] {
			next := slices.IndexFunc(others[at+1:], func(o Value) bool { return ctx.equalValues(o, elem) })
			if next < 0 {
				at = -1
				break
			}
			at += 1 + next
		}
		if at >= 0 {
			used[at] = true
			shared++
		}
	}
	return shared
}

// partialBinding reports a binding not determining the target whole: an end linking fewer values
// than its feature holds (KerML 1.0 §7.4.9.2), a connector declaring fewer links (`binding [1]`) than
// an end's feature holds, or one reaching the target across a collection.
func (ctx *Context) partialBinding(owner, targetInst *Instance, target *FeatureValue,
	binding lower.Binding, key featureValueRef) (partial bool, err error) {
	defer ctx.claimBinding(key, binding.Origin)()
	for end := range binding.Ends {
		endpoint, err := ctx.resolveBindingEndpoint(owner, binding, end)
		if err != nil {
			return false, err
		}
		if !endpoint.spread() || !endpoint.carries(target, targetInst) {
			continue
		}
		other, err := ctx.resolveBindingEndpoint(owner, binding, 1-end)
		if err != nil {
			return false, err
		}
		_, found, err := ctx.bindingEndpointValue(other, owner, endOwn)
		if err != nil || !found {
			return false, err
		}
		return true, nil
	}
	links, linksOK := ctx.model.semantics.RangeIn(binding.Scope, binding.Multiplicity)
	for end := range binding.Ends {
		stated, statedOK := ctx.model.semantics.RangeIn(binding.Scope, binding.Ends[end].Multiplicity)
		// The binding links at most the smaller finite upper bound of the end's own
		// multiplicity and the connector's (`binding [1]` declares one link).
		var upper int64
		bounded := false
		if statedOK && stated.Upper.Known && !stated.Upper.Infinite {
			upper, bounded = stated.Upper.Value, true
		}
		if linksOK && links.Upper.Known && !links.Upper.Infinite && (!bounded || links.Upper.Value < upper) {
			upper, bounded = links.Upper.Value, true
		}
		required := statedOK && stated.Lower.Known && stated.Lower.Value > 0
		if !bounded && !required {
			continue
		}
		endpoint, err := ctx.resolveBindingEndpoint(owner, binding, end)
		if err != nil {
			return false, err
		}
		if endpoint.expr != nil {
			continue
		}
		narrows := bounded && endpointAdmitsMoreThan(endpoint, upper)
		// A feature that must hold more values than the binding links can never be
		// linked whole, so the binding is partial without reading the end's value.
		if narrows && endpointRequiresMoreThan(endpoint, upper) {
			partial = true
			continue
		}
		if !narrows && !required {
			continue
		}
		// The end being resolved counts what it holds on its own: reading it through
		// its bindings is this very resolution, and cannot make the binding whole.
		carries := endpoint.carries(target, targetInst)
		var val Value
		var found bool
		if carries {
			val, found, err = ctx.ownEndpointValue(endpoint.locations[0])
		} else {
			val, found, err = ctx.bindingEndpointValue(endpoint, owner, endOwn)
		}
		if err != nil {
			return false, err
		}
		count := int64(len(elementsOf(val)))
		if found && required && count < stated.Lower.Value {
			return false, ctx.bindingEndCountError(binding, end, stated, count)
		}
		if narrows && (carries || !found || count == 0 || count > upper) {
			partial = true
		}
	}
	return partial, nil
}

// endpointRequiresMoreThan reports whether the features an end reaches must hold
// more than n values together, by their declared lower bounds.
func endpointRequiresMoreThan(endpoint bindingEndpoint, n int64) bool {
	var total int64
	for _, loc := range endpoint.locations {
		declared := loc.instance.FeatureValues[loc.name].Feature.Multiplicity.Lower
		if declared.Known {
			total += declared.Value
		}
	}
	return total > n
}

// endpointAdmitsMoreThan reports whether the features an end reaches may hold more than n
// values together, by their declared upper bounds.
func endpointAdmitsMoreThan(endpoint bindingEndpoint, n int64) bool {
	var total int64
	for _, loc := range endpoint.locations {
		declared := loc.instance.FeatureValues[loc.name].Feature.Multiplicity.Upper
		if declared.Infinite {
			return true
		}
		if declared.Known {
			total += declared.Value
		}
	}
	return total > n
}

// ownEndpointValue is what a feature holds without resolving its bindings: its
// written, defaulted or already-assigned value.
func (ctx *Context) ownEndpointValue(loc bindingLocation) (Value, bool, error) {
	fv := loc.instance.FeatureValues[loc.name]
	if !fv.Materialized && !fv.BindingDerived && ctx.CompositeTypeOf(fv.Feature) == nil {
		if _, err := loc.instance.materializeFeatureValueIntrinsic(ctx, loc.name); err != nil {
			return Value{}, false, &FeatureValueError{Err: err, Instance: loc.instance, Feature: loc.name}
		}
	}
	ctx.noteRead(loc.instance, fv)
	val := fv.HeldValue()
	return val, val.Kind != ValInvalid, nil
}

// wholeBindingCounts checks the value a whole binding identifies its ends with
// against the number of values each end states it links.
func (ctx *Context) wholeBindingCounts(binding lower.Binding, val Value) error {
	count := int64(len(elementsOf(val)))
	for end := range binding.Ends {
		stated, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Ends[end].Multiplicity)
		if !ok {
			continue
		}
		if (stated.Lower.Known && count < stated.Lower.Value) ||
			(stated.Upper.Known && !stated.Upper.Infinite && count > stated.Upper.Value) {
			return ctx.bindingEndCountError(binding, end, stated, count)
		}
	}
	// The connector's own multiplicity states how many links the binding declares.
	if links, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Multiplicity); ok {
		if (links.Lower.Known && count < links.Lower.Value) ||
			(links.Upper.Known && !links.Upper.Infinite && count > links.Upper.Value) {
			return fmt.Errorf("%w: `%s` declares %s link(s) but identifies %d value(s)",
				ErrMultiplicityViolation, ctx.bindingText(binding), links.Text(), count)
		}
	}
	return nil
}

func (ctx *Context) bindingEndCountError(binding lower.Binding, end int, stated semantics.Range, count int64) error {
	return fmt.Errorf("%w: `%s` links %s of %s, which holds %d value(s)",
		ErrMultiplicityViolation, ctx.bindingText(binding), stated.Text(),
		ctx.bindingEndpointText(binding, end), count)
}

// bindingText spells a binding as written, connector and end multiplicities included.
func (ctx *Context) bindingText(binding lower.Binding) string {
	ends := make([]string, len(binding.Ends))
	for i, end := range binding.Ends {
		ends[i] = ctx.bindingEndpointText(binding, i)
		if r, ok := ctx.model.semantics.RangeIn(binding.Scope, end.Multiplicity); ok {
			ends[i] = r.Text() + " " + ends[i]
		}
	}
	text := "bind " + strings.Join(ends, " = ")
	if r, ok := ctx.model.semantics.RangeIn(binding.Scope, binding.Multiplicity); ok {
		text = "binding " + r.Text() + " " + text
	}
	return text
}

func bindingEndForPath(binding lower.Binding, path string) int {
	for end, endpoint := range binding.Ends {
		if endpoint.HolderPath() == path {
			return end
		}
	}
	return -1
}

func (ctx *Context) attemptBinding(owner, targetInst *Instance, target *FeatureValue,
	binding lower.Binding, key featureValueRef, carryingRead endRead) (attempt bindingAttempt) {
	defer ctx.claimBinding(key, binding.Origin)()
	attempt.defaulted = binding.Ends[0].Default || binding.Ends[1].Default

	left, err := ctx.resolveBindingEndpoint(owner, binding, 0)
	if err != nil {
		attempt.err = err
		return attempt
	}
	right, err := ctx.resolveBindingEndpoint(owner, binding, 1)
	if err != nil {
		attempt.err = err
		return attempt
	}
	leftCarries := left.carries(target, targetInst)
	rightCarries := right.carries(target, targetInst)
	if !leftCarries && !rightCarries {
		return attempt
	}
	// An end reaching the target among several objects binds their values together, and
	// so determines no one object's part: partialBinding reports the undetermined case.
	if (leftCarries && left.spread()) || (rightCarries && right.spread()) {
		return attempt
	}
	attempt.settled = !ctx.resolvingBindingEnd(left, leftCarries) && !ctx.resolvingBindingEnd(right, rightCarries)
	// A `default =` value applies only to a feature with no value of its own (SysML v2 §7.13.4).
	if carrying, other := left, 1; true {
		if rightCarries {
			carrying, other = right, 0
		}
		if binding.Ends[other].Default {
			if endpointDeclaresValue(carrying) {
				attempt.settled = false
				return attempt
			}
		}
	}

	var leftValue, rightValue Value
	var leftSet, rightSet bool
	if leftCarries {
		leftValue, leftSet, rightValue, rightSet, err = ctx.readBindingEnds(owner, left, right, rightCarries, carryingRead)
	} else {
		rightValue, rightSet, leftValue, leftSet, err = ctx.readBindingEnds(owner, right, left, leftCarries, carryingRead)
	}
	if err != nil {
		attempt.err = err
		return attempt
	}
	for _, endpoint := range []bindingEndpoint{left, right} {
		for _, loc := range endpoint.locations {
			if ctx.genuineBindingCycle(featureValueRef{instance: loc.instance.ID, feature: loc.name}, binding.Origin) {
				attempt.cycle = true
			}
		}
	}
	if attempt.cycle {
		attempt.cycleFeatures = []string{
			ctx.bindingEndpointText(binding, 0),
			ctx.bindingEndpointText(binding, 1),
		}
	}

	// A binding found through the feature an end selects from checks the selected element
	// against the other end and the stated link counts, and determines nothing: no value is
	// assigned through an index. Ends holding nothing are left to the unmet-binding check.
	if (leftCarries && left.selects()) || (rightCarries && right.selects()) {
		if leftSet && rightSet && !ctx.equalValues(leftValue, rightValue) {
			attempt.err = &bindingConflictError{
				Left: ctx.bindingEndpointText(binding, 0), Right: ctx.bindingEndpointText(binding, 1),
				LeftValue: leftValue, RightValue: rightValue,
			}
			return attempt
		}
		if leftSet || rightSet {
			identified := leftValue
			if !leftSet {
				identified = rightValue
			}
			attempt.err = ctx.wholeBindingCounts(binding, identified)
			attempt.settled = false
		}
		return attempt
	}

	switch {
	case leftSet && rightSet:
		leftDerived := bindingEndpointDerived(left)
		rightDerived := bindingEndpointDerived(right)
		if leftDerived && !rightDerived {
			attempt.value = rightValue
			attempt.found = true
			attempt.assignment = &bindingAssignment{
				endpoint: left, value: rightValue, binding: binding, end: 0,
			}
			return attempt
		}
		if rightDerived && !leftDerived {
			attempt.value = leftValue
			attempt.found = true
			attempt.assignment = &bindingAssignment{
				endpoint: right, value: leftValue, binding: binding, end: 1,
			}
			return attempt
		}
		if !ctx.equalValues(leftValue, rightValue) {
			attempt.err = &bindingConflictError{
				Left: ctx.bindingEndpointText(binding, 0), Right: ctx.bindingEndpointText(binding, 1),
				LeftValue: leftValue, RightValue: rightValue,
			}
			return attempt
		}
		attempt.value = leftValue
		attempt.found = true
	case leftSet:
		attempt.assignment = &bindingAssignment{
			endpoint: right, value: leftValue, binding: binding, end: 1,
		}
		if leftCarries || rightCarries {
			attempt.value = leftValue
			attempt.found = true
		}
	case rightSet:
		attempt.assignment = &bindingAssignment{
			endpoint: left, value: rightValue, binding: binding, end: 0,
		}
		if leftCarries || rightCarries {
			attempt.value = rightValue
			attempt.found = true
		}
	}
	// The objects a spread end reaches keep their own parts of the bound values, and a
	// selecting end's feature keeps its own.
	if attempt.assignment != nil && (attempt.assignment.endpoint.spread() || attempt.assignment.endpoint.selects()) {
		attempt.assignment = nil
	}
	return attempt
}

// readBindingEnds reads the other end first, through its own bindings, so what it holds is what the
// carrying end becomes; a spread other end is materialized once the carrying end has nothing of its own.
func (ctx *Context) readBindingEnds(owner *Instance, carrying, other bindingEndpoint, otherCarries bool, carryingRead endRead,
) (carryingValue Value, carryingSet bool, otherValue Value, otherSet bool, err error) {
	otherRead := endBound
	if otherCarries {
		otherRead = endMaterialized
	}
	otherValue, otherSet, err = ctx.bindingEndpointValue(other, owner, otherRead)
	if err != nil {
		return
	}
	if !otherSet && other.spread() {
		defaultedRead := endOwn
		if endpointDefaulted(carrying) {
			defaultedRead = carryingRead
		}
		carryingValue, carryingSet, err = ctx.bindingEndpointValue(carrying, owner, defaultedRead)
		if err != nil || carryingSet {
			return
		}
		otherValue, otherSet, err = ctx.bindingEndpointValue(other, owner, endMaterialized)
		return
	}
	if otherSet && carryingRead == endMaterialized {
		carryingRead = endBound
	}
	carryingValue, carryingSet, err = ctx.bindingEndpointValue(carrying, owner, carryingRead)
	return
}

// endpointDeclaresValue reports a feature end whose every feature states a value of its own,
// by its declaration or by a write.
func endpointDeclaresValue(endpoint bindingEndpoint) bool {
	for _, loc := range endpoint.locations {
		fv := loc.instance.FeatureValues[loc.name]
		if fv.Feature.DefaultValue == nil && !fv.Written {
			return false
		}
	}
	return len(endpoint.locations) != 0
}

// endpointDefaulted reports a feature end whose every feature declares a default value.
func endpointDefaulted(endpoint bindingEndpoint) bool {
	for _, loc := range endpoint.locations {
		if loc.instance.FeatureValues[loc.name].Feature.DefaultValue == nil {
			return false
		}
	}
	return len(endpoint.locations) != 0
}

// resolvingBindingEnd reports an end other than the one being resolved whose own
// resolution is in progress, so what it holds is not yet known.
func (ctx *Context) resolvingBindingEnd(endpoint bindingEndpoint, carries bool) bool {
	if carries {
		return false
	}
	for _, loc := range endpoint.locations {
		if ctx.resolvingBindings[featureValueRef{instance: loc.instance.ID, feature: loc.name}] {
			return true
		}
	}
	return false
}

func (ctx *Context) genuineBindingCycle(ref featureValueRef, binding program.Origin) bool {
	return ctx.resolvingBindings[ref] && ctx.bindingOwners[ref] != binding
}

func bindingInvolvesFeature(binding lower.Binding, name string) bool {
	for _, end := range binding.Ends {
		if end.HolderPath() == name {
			return true
		}
	}
	return false
}

func (ctx *Context) resolveBindingEndpoint(owner *Instance, binding lower.Binding, end int) (bindingEndpoint, error) {
	path := binding.Ends[end].Path
	if path == "" {
		if binding.Ends[end].Expr == nil {
			return bindingEndpoint{}, fmt.Errorf("%w: empty endpoint", errBindingEnd)
		}
		endpoint := bindingEndpoint{expr: binding.Ends[end].Expr, scope: binding.Scope}
		if via := binding.Ends[end].Via; len(via) != 0 {
			on, err := ctx.bindingEndpointObjects(owner, via)
			if err != nil {
				return bindingEndpoint{}, err
			}
			endpoint.on = on
		}
		return endpoint, nil
	}
	on := []*Instance{owner}
	if via := binding.Ends[end].Via; len(via) != 0 {
		var err error
		if on, err = ctx.bindingEndpointObjects(owner, via); err != nil {
			return bindingEndpoint{}, err
		}
	}
	endpoint := bindingEndpoint{index: binding.Ends[end].Index, selection: binding.Ends[end].Expr, scope: binding.Scope}
	for _, obj := range on {
		reached, err := ctx.resolveBindingLocations(obj, path)
		if err != nil {
			return bindingEndpoint{}, err
		}
		endpoint.locations = append(endpoint.locations, reached...)
		endpoint.reached = append(endpoint.reached, reached)
	}
	if len(binding.Ends[end].Via) != 0 {
		endpoint.on = on
	} else {
		endpoint.reached = nil
	}
	return endpoint, nil
}

// withBoundMembers adds the Kernel frame members the model binds through feature
// (`cf.edges = ae`) to the object's features, so each holds a value like any other.
func (ctx *Context) withBoundMembers(inst, owner *Instance, feature string, features []EffectiveFeature) []EffectiveFeature {
	if owner == nil || feature == "" {
		return features
	}
	types := inst.types()
	held := make(map[string]bool, len(features))
	for i := range features {
		held[features[i].Name] = true
	}
	added := false
	path := feature
	for current := owner; current != nil; {
		for _, name := range ctx.membersBoundThrough(current, path) {
			if held[name] {
				continue
			}
			member, typ := ctx.boundFrameMember(types, name)
			if member == nil {
				continue
			}
			if !added {
				features, added = slices.Clone(features), true
			}
			held[name] = true
			features = append(features, ctx.effectiveFeature(name, member, typ))
		}
		next, nextFeature := current.Owner()
		if next == nil || nextFeature == "" {
			break
		}
		path = nextFeature + "." + path
		current = next
	}
	return features
}

// membersBoundThrough names the members of the object at path below owner that
// the bindings of owner's types reach directly, in binding order.
func (ctx *Context) membersBoundThrough(owner *Instance, path string) []string {
	var names []string
	for _, typ := range owner.types() {
		for _, binding := range ctx.objectBindings(typ) {
			for _, end := range binding.Ends {
				if name, ok := strings.CutPrefix(end.HolderPath(), path+"."); ok && !strings.Contains(name, ".") {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

// boundFrameMember finds the Kernel frame member name of the first of types
// declaring it that the shape leaves out, with that type.
func (ctx *Context) boundFrameMember(types []*symbols.Symbol, name string) (*symbols.Symbol, *symbols.Symbol) {
	sem := ctx.model.semantics
	for _, typ := range types {
		member, found := sem.LookupMember(typ, name)
		if found && member.IsFeature() && sem.FrameFeature(member) && !sem.FrameRoot(typ, member) {
			return member, typ
		}
	}
	return nil, nil
}

// resolveBindingLocations follows an end's path from owner to the named feature on every
// object it reaches, visiting each object a collection-valued step holds, in order.
func (ctx *Context) resolveBindingLocations(owner *Instance, path string) ([]bindingLocation, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("%w: empty endpoint", errBindingEnd)
	}
	reached := []*Instance{owner}
	for _, part := range parts[:len(parts)-1] {
		var next []*Instance
		for _, current := range reached {
			fv, err := current.GetFeatureValue(ctx, part)
			if err != nil {
				return nil, fmt.Errorf("%w %q: %w", errBindingEnd, path, err)
			}
			heldValues, err := ctx.HeldElements(fv.HeldValue())
			if err != nil {
				return nil, fmt.Errorf("%w %q: %w", errBindingEnd, path, err)
			}
			for _, held := range heldValues {
				id, isObject := held.Object()
				if !isObject {
					return nil, fmt.Errorf("%w %q: %s is not an object", errBindingEnd, path, part)
				}
				obj, ok, err := ctx.reachedObject(id)
				if err != nil {
					return nil, fmt.Errorf("%w %q: %w", errBindingEnd, path, err)
				}
				if !ok {
					return nil, fmt.Errorf("%w %q: instance %d is not materialized", errBindingEnd, path, id)
				}
				next = append(next, obj)
			}
		}
		reached = next
	}
	name := parts[len(parts)-1]
	locations := make([]bindingLocation, 0, len(reached))
	for _, current := range reached {
		// A destroyed object's feature is neither read into nor written from a
		// binding, whichever end of it the object is.
		if err := ctx.checkNotDestroyed(current); err != nil {
			return nil, fmt.Errorf("%w %q: %w", errBindingEnd, path, err)
		}
		if _, ok := current.FeatureValues[name]; !ok {
			return nil, fmt.Errorf("%w %q: feature %s not found", errBindingEnd, path, name)
		}
		locations = append(locations, bindingLocation{instance: current, name: name, path: path})
	}
	return locations, nil
}

// expressionEndValue evaluates an expression end on the binding's owner, or on each connector
// object it is read on: every connector states the same binding, so their values must agree.
func (ctx *Context) expressionEndValue(endpoint bindingEndpoint, owner *Instance) (Value, bool, error) {
	on := endpoint.on
	if len(on) == 0 {
		on = []*Instance{owner}
	}
	var agreed Value
	for i, obj := range on {
		value, err := ctx.EvalWithScopeOn(endpoint.expr, endpoint.scope, obj)
		if err != nil {
			if errors.Is(err, errUninitializedFeatureValue) {
				return Value{}, false, nil
			}
			return Value{}, false, fmt.Errorf("%w: expression %s: %w",
				errBindingEnd, ctx.bindingExprText(endpoint.expr, endpoint.scope), err)
		}
		if value.Kind == ValInvalid {
			return Value{}, false, nil
		}
		if i != 0 && !ctx.equalValues(agreed, value) {
			text := ctx.bindingExprText(endpoint.expr, endpoint.scope)
			return Value{}, false, &bindingConflictError{Left: text, Right: text, LeftValue: agreed, RightValue: value}
		}
		agreed = value
	}
	return agreed, true, nil
}

// boundValueOf is what a feature's own bindings determine it holds, creating no object for it, and
// nothing while its own resolution is in progress (it then reads as undetermined, not as a cycle).
func (ctx *Context) boundValueOf(loc bindingLocation) (Value, bool, error) {
	if ctx.resolvingBindings[featureValueRef{instance: loc.instance.ID, feature: loc.name}] {
		return Value{}, false, nil
	}
	val, found, err := ctx.resolveBindingValue(loc.instance, loc.name)
	if err != nil || !found {
		return Value{}, false, err
	}
	return val, true, nil
}

// bindingEndpointObjects are the connector objects via leads to from owner, hop by hop: every
// object a named connector feature holds, or every anonymous connector a declaration materialized.
func (ctx *Context) bindingEndpointObjects(owner *Instance, via []lower.Hop) ([]*Instance, error) {
	current := []*Instance{owner}
	for _, hop := range via {
		var next []*Instance
		for _, obj := range current {
			reached, err := ctx.connectorObjectsAt(obj, hop)
			if err != nil {
				return nil, err
			}
			next = append(next, reached...)
		}
		current = next
	}
	if len(current) == 0 {
		return nil, fmt.Errorf("%w: connector holds no object", errBindingEnd)
	}
	return current, nil
}

func (ctx *Context) connectorObjectsAt(owner *Instance, hop lower.Hop) ([]*Instance, error) {
	if hop.Anonymous != nil {
		connectors, err := owner.OwnedConnectors(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: anonymous connector: %w", errBindingEnd, err)
		}
		var out []*Instance
		for _, conn := range connectors {
			if conn.Type != nil && conn.Type.Decl == hop.Anonymous {
				out = append(out, conn)
			}
		}
		return out, nil
	}
	fv, err := owner.GetFeatureValue(ctx, hop.Feature)
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", errBindingEnd, hop.Feature, err)
	}
	held, err := ctx.HeldElements(fv.HeldValue())
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", errBindingEnd, hop.Feature, err)
	}
	out := make([]*Instance, 0, len(held))
	for _, value := range held {
		id, isObject := value.Object()
		if !isObject {
			return nil, fmt.Errorf("%w %q: holds no object", errBindingEnd, hop.Feature)
		}
		obj, ok, err := ctx.reachedObject(id)
		if err != nil {
			return nil, fmt.Errorf("%w %q: %w", errBindingEnd, hop.Feature, err)
		}
		if !ok {
			return nil, fmt.Errorf("%w %q: instance %d is not materialized", errBindingEnd, hop.Feature, id)
		}
		out = append(out, obj)
	}
	return out, nil
}

// bindingEndpointValue is what an end holds: its expression's or feature's value, or a spread end's
// values together in object order — nothing while one of them is undetermined.
func (ctx *Context) bindingEndpointValue(endpoint bindingEndpoint, owner *Instance, read endRead) (Value, bool, error) {
	if endpoint.selects() {
		return ctx.selectedBindingElement(endpoint, owner, read)
	}
	if endpoint.expr != nil {
		return ctx.expressionEndValue(endpoint, owner)
	}
	if !endpoint.spread() {
		return ctx.bindingLocationValue(endpoint.locations[0], read)
	}
	var all []Value
	for _, loc := range endpoint.locations {
		val, found, err := ctx.bindingLocationValue(loc, read)
		var undetermined *undeterminedBindingError
		if errors.As(err, &undetermined) {
			return Value{}, false, nil
		}
		if err != nil || !found {
			return Value{}, false, err
		}
		all = append(all, elementsOf(val)...)
	}
	if err := ctx.chargeElements(int64(len(all))); err != nil {
		return Value{}, false, err
	}
	return sequenceOf(all), true, nil
}

// selectedBindingElement is the element at the 1-based index a selecting end states, read as `#(`
// reads it on the owner, or on each connector object from what the end reaches there; those agree.
func (ctx *Context) selectedBindingElement(endpoint bindingEndpoint, owner *Instance, read endRead) (Value, bool, error) {
	on, reached := endpoint.on, endpoint.reached
	if len(on) == 0 {
		on, reached = []*Instance{owner}, [][]bindingLocation{endpoint.locations}
	}
	var agreed Value
	for i, obj := range on {
		whole, found, err := ctx.bindingEndpointValue(bindingEndpoint{locations: reached[i]}, owner, read)
		if err != nil || !found {
			return Value{}, false, err
		}
		element, err := ctx.selectedElementOn(endpoint, obj, whole)
		if err != nil {
			return Value{}, false, err
		}
		if i != 0 && !ctx.equalValues(agreed, element) {
			text := ctx.bindingExprText(endpoint.selection, endpoint.scope)
			return Value{}, false, &bindingConflictError{Left: text, Right: text, LeftValue: agreed, RightValue: element}
		}
		agreed = element
	}
	return agreed, true, nil
}

// selectedElementOn is the element of whole at the index the end states, evaluated on obj; a
// feature holding no element there is a binding end error.
func (ctx *Context) selectedElementOn(endpoint bindingEndpoint, obj *Instance, whole Value) (Value, error) {
	indexVal, err := ctx.EvalWithScopeOn(endpoint.index, endpoint.scope, obj)
	if err == nil {
		const op = "sequence index"
		var index int64
		if index, err = indexOf(op, indexVal); err == nil {
			var element Value
			if element, err = ctx.positionOf(op, whole, index); err == nil {
				return element, nil
			}
		}
	}
	return Value{}, fmt.Errorf("%w: expression %s: %w",
		errBindingEnd, ctx.bindingExprText(endpoint.selection, endpoint.scope), err)
}

func (ctx *Context) bindingLocationValue(loc bindingLocation, read endRead) (Value, bool, error) {
	fv := loc.instance.FeatureValues[loc.name]
	if fv.BindingDerived && read == endMaterialized {
		return Value{}, false, nil
	}
	if fv.BindingDerived {
		if ctx.CompositeTypeOf(fv.Feature) != nil {
			if val := fv.HeldValue(); val.Kind != ValInvalid {
				ctx.noteRead(loc.instance, fv)
				return val, true, nil
			}
		}
		return ctx.boundValueOf(loc)
	}
	if !fv.Materialized {
		if ctx.CompositeTypeOf(fv.Feature) != nil {
			if read == endBound {
				return ctx.boundValueOf(loc)
			}
			if read == endOwn {
				return Value{}, false, nil
			}
		}
		if _, err := loc.instance.materializeFeatureValueIntrinsic(ctx, loc.name); err != nil {
			return Value{}, false, &FeatureValueError{Err: err, Instance: loc.instance, Feature: loc.name}
		}
	}
	ctx.noteRead(loc.instance, fv)
	if val := fv.HeldValue(); val.Kind != ValInvalid {
		return val, true, nil
	}
	if ctx.resolvingBindings[featureValueRef{instance: loc.instance.ID, feature: loc.name}] {
		return Value{}, false, nil
	}
	val, found, err := ctx.resolveBindingValue(loc.instance, loc.name)
	if err != nil {
		return Value{}, false, err
	}
	return val, found, nil
}

// endRead is how far a binding end is read: what it holds or its declaration states (endOwn),
// that and what its own bindings determine, creating no object (endBound), or materialized.
type endRead int

const (
	endOwn endRead = iota
	endBound
	endMaterialized
)

// bindingEndpointDerived reports a feature end whose every value was assigned by a binding.
func bindingEndpointDerived(endpoint bindingEndpoint) bool {
	if endpoint.selects() {
		return false
	}
	for _, loc := range endpoint.locations {
		fv := loc.instance.FeatureValues[loc.name]
		if !fv.BindingDerived || fv.Written {
			return false
		}
	}
	return len(endpoint.locations) != 0
}

func (ctx *Context) assignBindingEndpoint(endpoint bindingEndpoint, val Value, binding lower.Binding, end int) error {
	if endpoint.expr != nil {
		return fmt.Errorf("%w: cannot assign %s from %s",
			errBindingEnd, ctx.bindingEndpointText(binding, end), ctx.bindingEndpointText(binding, 1-end))
	}
	if endpoint.spread() {
		// The objects a spread end reaches each hold a part of the value; which part is not stated.
		return fmt.Errorf("%w: cannot assign %s from %s through every object %s holds",
			errBindingEnd, ctx.bindingEndpointText(binding, end), ctx.bindingEndpointText(binding, 1-end), endpoint.across())
	}
	loc := endpoint.locations[0]
	fv := loc.instance.FeatureValues[loc.name]
	endWrite := ctx.beginFeatureWrite(fv)
	defer endWrite()
	before := ctx.beforeWrite(fv)
	err := ctx.assignBindingValue(loc.instance, fv, loc.name, val)
	ctx.afterWrite(fv, before)
	return err
}

func (ctx *Context) assignBindingValue(inst *Instance, fv *FeatureValue, name string, val Value) error {
	if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
		return err
	}
	val, err := ctx.holdDeclared(inst, fv, val)
	if err != nil {
		return err
	}
	ctx.noteProbeWrite(fv)
	ctx.noteFeatureWrite(fv)
	held := fv.HeldValue()
	if fv.Feature.Scalar() {
		fv.Value = val
		fv.Values = Value{}
	} else {
		fv.Value = Value{}
		fv.Values = val
	}
	fv.Materialized, fv.intrinsic = true, false
	fv.BindingDerived, fv.Assumed = true, false
	ctx.adoptOccurrenceWrite(inst, fv)
	ctx.releaseStandIns(held)
	return nil
}

func bindingLocationCarries(loc bindingLocation, fv *FeatureValue, inst *Instance) bool {
	return loc.instance == inst && loc.instance.FeatureValues[loc.name] == fv
}

func bindingLocationText(loc bindingLocation) string {
	if loc.instance == nil {
		return loc.name
	}
	if loc.path != "" {
		return loc.path
	}
	if loc.instance.Type == nil {
		return loc.name
	}
	return fmt.Sprintf("%s.%s", loc.instance.Type.Name, loc.name)
}

func (ctx *Context) bindingEndpointText(binding lower.Binding, end int) string {
	endpoint := binding.Ends[end]
	if endpoint.Path != "" && endpoint.Index == nil {
		return endpoint.Path
	}
	return ctx.bindingExprText(endpoint.Expr, binding.Scope)
}

// ExprText renders an expression as the model wrote it, from the document's
// text where this context was given it, else from its structure.
func (ctx *Context) ExprText(expr ast.Node, scope *symbols.Scope) string {
	return ctx.bindingExprText(expr, scope)
}

func (ctx *Context) bindingExprText(expr ast.Node, scope *symbols.Scope) string {
	if expr == nil {
		return "<empty>"
	}
	if scope != nil && scope.Owner() != nil {
		if sf := ctx.model.sources[scope.Owner().DocName]; sf != nil {
			if text := strings.TrimSpace(sf.Text(expr.Span())); text != "" {
				return text
			}
		}
	}
	return ast.ExprPrinter{Other: ast.Dump}.Text(expr)
}

func bindingValueText(val Value) string {
	return FormatValue(val)
}
