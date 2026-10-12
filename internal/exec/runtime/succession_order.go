package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	successionOrderCycleCode    = "succession-order-cycle"
	successionOrderViolatedCode = "succession-order-violated"
	successionOrdersNothingCode = "succession-orders-nothing"
)

var (
	// ErrSuccessionOrderCycle identifies held behaviors waiting on each other.
	ErrSuccessionOrderCycle = errors.New("succession order cycle")
	// ErrSuccessionOrderViolated identifies an explicit start that breaks an order.
	ErrSuccessionOrderViolated = errors.New("succession order violated")
)

// SuccessionOrderError carries the runtime code and message for a refused start.
type SuccessionOrderError struct {
	Code    string
	Message string
	Err     error
}

func (e *SuccessionOrderError) Error() string { return e.Message }
func (e *SuccessionOrderError) Unwrap() error { return e.Err }

// SuccessionOrdersNothing reports a runtime succession that cannot order performances.
type SuccessionOrdersNothing struct {
	Reason string
	File   string
	Span   source.Span
}

func (n SuccessionOrdersNothing) Describe() string { return n.Reason }
func (n SuccessionOrdersNothing) String() string {
	return successionOrdersNothingCode + ": " + n.Reason
}
func (n SuccessionOrdersNothing) Location() (string, source.Span) {
	return n.File, n.Span
}
func (n SuccessionOrdersNothing) Diagnostic() diag.Diagnostic {
	return diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     n.Span,
		Message:  n.Reason,
		Code:     successionOrdersNothingCode,
		Source:   "runtime",
	}
}

type successionOrderBlock struct {
	order       lower.BehaviorOrder
	featuring   *Instance
	predecessor *ObjectBehavior
}

type successionOrderNoteKey struct {
	order  ast.Node
	object int64
}

func (ctx *Context) behaviorOrders() []lower.BehaviorOrder {
	if ctx.model.behaviorOrdersReady {
		return ctx.model.behaviorOrders
	}
	var roots []*symbols.Scope
	roots = append(roots, ctx.model.scopes...)
	if len(roots) == 0 && ctx.model.resolver != nil && ctx.model.resolver.Index() != nil {
		index := ctx.model.resolver.Index()
		for _, document := range index.Documents() {
			roots = append(roots, index.DocumentRoot(document))
		}
	}
	var base *symbols.Index
	if ctx.model.resolver != nil {
		if index := ctx.model.resolver.Index(); index != nil {
			if index.Frozen() {
				base = index
			} else if candidate := index.Base(); candidate != nil && candidate.Frozen() {
				base = candidate
			}
		}
	}
	var orders []lower.BehaviorOrder
	if base == nil {
		orders = lower.BehaviorOrders(ctx.model.semantics, roots...)
	} else {
		cached := libraryBehaviorOrders(base)
		for _, root := range roots {
			if cachedOrders, ok := cached[root]; ok {
				orders = append(orders, cachedOrders...)
				continue
			}
			orders = append(orders, lower.BehaviorOrders(ctx.model.semantics, root)...)
		}
	}
	ctx.model.behaviorOrders = orders
	ctx.model.behaviorOrdersByEnd = make(map[*symbols.Symbol][]lower.BehaviorOrder)
	for _, order := range ctx.model.behaviorOrders {
		if len(order.Earlier.Path) > 0 {
			end := order.Earlier.Path[len(order.Earlier.Path)-1]
			ctx.model.behaviorOrdersByEnd[end] = append(ctx.model.behaviorOrdersByEnd[end], order)
		}
		if len(order.Later.Path) > 0 {
			end := order.Later.Path[len(order.Later.Path)-1]
			ctx.model.behaviorOrdersByEnd[end] = append(ctx.model.behaviorOrdersByEnd[end], order)
		}
	}
	ctx.model.behaviorOrdersReady = true
	return ctx.model.behaviorOrders
}

func (ctx *Context) behaviorOrdersFor(member *symbols.Symbol, inst *Instance, later bool) []lower.BehaviorOrder {
	if member == nil {
		return nil
	}
	ctx.behaviorOrders()
	candidates := map[ast.Node]bool{}
	ends := map[*symbols.Symbol]bool{member: true}
	for _, owner := range inst.types() {
		for _, target := range ctx.redefinedFeatures(member, owner) {
			ends[target] = true
		}
	}
	for end := range ends {
		for _, order := range ctx.model.behaviorOrdersByEnd[end] {
			candidates[order.Decl] = true
		}
	}
	var orders []lower.BehaviorOrder
	for _, order := range ctx.model.behaviorOrders {
		if !candidates[order.Decl] || order.Refusal != nil {
			continue
		}
		path := order.Earlier.Path
		if later {
			path = order.Later.Path
		}
		if len(path) > 0 && ctx.behaviorOrderFeatureMatches(inst, member, path[len(path)-1]) {
			orders = append(orders, order)
		}
	}
	return orders
}

func (ctx *Context) behaviorOrderFeatureMatches(inst *Instance, member, target *symbols.Symbol) bool {
	if member == target {
		return true
	}
	owners := []*symbols.Symbol{nil}
	if inst != nil {
		owners = inst.types()
	}
	for _, owner := range owners {
		if slices.Contains(ctx.redefinedFeatures(member, owner), target) {
			return true
		}
	}
	return false
}

func (ctx *Context) orderFeaturingInstances(order lower.BehaviorOrder, end lower.BehaviorOrderEnd) []*Instance {
	if len(end.Path) == 0 {
		return nil
	}
	if order.Featuring == nil {
		instances, _ := ctx.liveOccurrences(end.Path[0])
		return sortedInstancesByID(instances)
	}
	if symbolIsUsage(order.Featuring) {
		instances, _ := ctx.liveOccurrences(order.Featuring)
		return sortedInstancesByID(instances)
	}
	var instances []*Instance
	for _, inst := range ctx.instances {
		if ctx.instanceConforms(inst, order.Featuring) {
			instances = append(instances, inst)
		}
	}
	sortInstancesByID(instances)
	return instances
}

func sortedInstancesByID(instances []*Instance) []*Instance {
	sorted := slices.Clone(instances)
	sortInstancesByID(sorted)
	return sorted
}

func sortInstancesByID(instances []*Instance) {
	slices.SortFunc(instances, func(a, b *Instance) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

func (ctx *Context) orderEndTargetsFrom(order lower.BehaviorOrder, end lower.BehaviorOrderEnd, featuring *Instance) []*Instance {
	if order.Featuring == nil {
		if len(end.Path) == 1 {
			return []*Instance{featuring}
		}
		return ctx.walkOrderFeaturePath([]*Instance{featuring}, end.Path[1:len(end.Path)-1])
	}
	return ctx.walkOrderFeaturePath([]*Instance{featuring}, end.Path[:len(end.Path)-1])
}

func (ctx *Context) orderEndTargetsForOrder(order lower.BehaviorOrder, end lower.BehaviorOrderEnd, featuring *Instance) []*Instance {
	if order.Featuring != nil || len(end.Path) == 0 {
		return ctx.orderEndTargetsFrom(order, end, featuring)
	}
	instances, _ := ctx.liveOccurrences(end.Path[0])
	if len(end.Path) == 1 {
		return instances
	}
	return ctx.walkOrderFeaturePath(instances, end.Path[1:len(end.Path)-1])
}

func (ctx *Context) walkOrderFeaturePath(instances []*Instance, path []*symbols.Symbol) []*Instance {
	for _, feature := range path {
		var next []*Instance
		seen := make(map[*Instance]bool)
		for _, inst := range instances {
			for _, child := range ctx.orderFeatureValues(inst, feature) {
				if !seen[child] {
					seen[child] = true
					next = append(next, child)
				}
			}
		}
		instances = next
	}
	return instances
}

func (ctx *Context) orderFeatureValues(inst *Instance, feature *symbols.Symbol) []*Instance {
	if inst == nil || feature == nil {
		return nil
	}
	var out []*Instance
	seen := make(map[*Instance]bool)
	var addValue func(Value)
	addValue = func(value Value) {
		if child, ok := value.Object(); ok {
			if object, exists := ctx.Instance(child); exists && !seen[object] {
				seen[object] = true
				out = append(out, object)
			}
		}
		if sequence := value.Sequence(); sequence != nil {
			for _, element := range sequence.Elements() {
				addValue(element)
			}
		}
		if set := value.Set(); set != nil {
			for _, element := range set.Elements() {
				addValue(element)
			}
		}
	}
	for _, fv := range inst.FeatureValues {
		if fv.Feature == nil || !ctx.behaviorOrderFeatureMatches(inst, fv.Feature.Symbol, feature) {
			continue
		}
		addValue(fv.HeldValue())
	}
	for _, child := range ctx.instances {
		owner, ownerFeature := child.Owner()
		if owner != inst {
			continue
		}
		fv := inst.FeatureValues[ownerFeature]
		if fv == nil || fv.Feature == nil || !ctx.behaviorOrderFeatureMatches(inst, fv.Feature.Symbol, feature) {
			continue
		}
		if !seen[child] {
			seen[child] = true
			out = append(out, child)
		}
	}
	sortInstancesByID(out)
	return out
}

func (ctx *Context) behaviorOrderMatches(inst *Instance, member *symbols.Symbol, order lower.BehaviorOrder, end lower.BehaviorOrderEnd) []*Instance {
	if len(end.Path) == 0 || !ctx.behaviorOrderFeatureMatches(inst, member, end.Path[len(end.Path)-1]) {
		return nil
	}
	var matching []*Instance
	for _, featuring := range ctx.orderFeaturingInstances(order, end) {
		if slices.Contains(ctx.orderEndTargetsFrom(order, end, featuring), inst) {
			matching = append(matching, featuring)
		}
	}
	return matching
}

func (ctx *Context) deferredBehaviorFor(inst *Instance, decl classifierBehaviorDecl, binding int, performance int64) (*ObjectBehavior, error) {
	chain, err := ctx.classifierBehaviorChain(decl)
	if err != nil {
		return nil, err
	}
	deferred := decl
	ctx.behaviorHeld()
	return &ObjectBehavior{
		Name:        decl.behavior.Name,
		Kind:        decl.behavior.Kind,
		Symbol:      chain[len(chain)-1],
		Object:      inst,
		member:      decl.member,
		bindings:    chain,
		kinds:       ctx.behaviorKinds(chain),
		binding:     binding,
		performance: performance,
		typeBound:   true,
		ctx:         ctx,
		deferred:    &deferred,
	}, nil
}

func (ctx *Context) shouldDeferBehavior(inst *Instance, member *symbols.Symbol) bool {
	for _, order := range ctx.behaviorOrdersFor(member, inst, true) {
		if order.Refusal == nil && len(ctx.behaviorOrderMatches(inst, member, order, order.Later)) > 0 {
			return true
		}
	}
	return false
}

func (ctx *Context) classifierBehaviorMemberForAction(action *symbols.Symbol, self *Instance) *symbols.Symbol {
	if action == nil || self == nil {
		return nil
	}
	for _, typ := range self.types() {
		for _, decl := range ctx.classifierBehaviorsOf(typ) {
			body, err := ctx.classifierBehaviorSymbol(decl)
			if err != nil {
				continue
			}
			if body == action || ctx.sameFeature(action, decl.member, typ) ||
				ctx.sameFeature(body, action, typ) {
				return decl.member
			}
		}
		for _, member := range ctx.model.semantics.MembersOf(typ) {
			if member.Decl == nil {
				continue
			}
			if _, ok := lower.StartableBehaviorOf(member.Decl); ok &&
				(action == member || ctx.sameFeature(action, member, typ)) {
				return member
			}
		}
	}
	return nil
}

func (ctx *Context) classifierBehaviorMemberForState(stateMachine *symbols.Symbol, self *Instance) *symbols.Symbol {
	if stateMachine == nil || self == nil {
		return nil
	}
	for _, typ := range self.types() {
		for _, decl := range ctx.classifierBehaviorsOf(typ) {
			if decl.behavior.Kind == lower.ExhibitedState && ctx.ExhibitsState(decl.member, stateMachine) {
				return decl.member
			}
		}
		for _, member := range ctx.model.semantics.MembersOf(typ) {
			if member.Decl == nil {
				continue
			}
			behavior, ok := lower.StartableBehaviorOf(member.Decl)
			if ok && behavior.Kind == lower.ExhibitedState &&
				(stateMachine == member || ctx.sameFeature(stateMachine, member, typ)) {
				return member
			}
		}
	}
	return nil
}

func (ctx *Context) behaviorOrderBlocks(behavior *ObjectBehavior) []successionOrderBlock {
	if behavior == nil || behavior.member == nil || ctx.lifeEnded(behavior.Object) {
		return nil
	}
	var blocks []successionOrderBlock
	seen := make(map[string]bool)
	for _, order := range ctx.behaviorOrdersFor(behavior.member, behavior.Object, true) {
		if order.Refusal != nil {
			continue
		}
		featuring := ctx.behaviorOrderMatches(behavior.Object, behavior.member, order, order.Later)
		if len(featuring) == 0 {
			continue
		}
		for _, instance := range featuring {
			if !ctx.laterEndIsUnique(order, instance, nil, nil) {
				continue
			}
			predecessors := ctx.behaviorOrderEndPerformances(order, order.Earlier, instance)
			key := fmt.Sprintf("%p:%p", order.Decl, instance)
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(predecessors) == 1 && !ctx.behaviorPerformanceEnded(predecessors[0]) {
				blocks = append(blocks, successionOrderBlock{
					order: order, featuring: instance, predecessor: predecessors[0],
				})
			}
		}
	}
	return blocks
}

func (ctx *Context) behaviorOrderEndPerformances(order lower.BehaviorOrder, end lower.BehaviorOrderEnd, featuring *Instance) []*ObjectBehavior {
	targets := ctx.orderEndTargetsForOrder(order, end, featuring)
	var performances []*ObjectBehavior
	for _, candidate := range ctx.objectBehaviors {
		if candidate == nil || candidate.member == nil ||
			!ctx.behaviorOrderFeatureMatches(candidate.Object, candidate.member, endFeature(end)) ||
			!slices.Contains(targets, candidate.Object) {
			continue
		}
		performances = append(performances, candidate)
	}
	return performances
}

func (ctx *Context) behaviorOrderEndPerformanceCount(
	order lower.BehaviorOrder, end lower.BehaviorOrderEnd, featuring, starting *Instance, member *symbols.Symbol,
) int {
	performances := ctx.behaviorOrderEndPerformances(order, end, featuring)
	count := len(performances)
	if starting == nil || member == nil ||
		!ctx.behaviorOrderFeatureMatches(starting, member, endFeature(end)) {
		return count
	}
	targets := ctx.orderEndTargetsForOrder(order, end, featuring)
	if !slices.Contains(targets, starting) {
		return count
	}
	for _, behavior := range performances {
		if behavior.Object == starting && behavior.deferred != nil {
			return count
		}
	}
	return count + 1
}

func (ctx *Context) laterEndIsUnique(order lower.BehaviorOrder, featuring, starting *Instance, member *symbols.Symbol) bool {
	count := ctx.behaviorOrderEndPerformanceCount(order, order.Later, featuring, starting, member)
	if count == 1 {
		return true
	}
	reason := fmt.Sprintf("%d later-end performances are present in one featuring instance; their pairing is open under KERML-29", count)
	ctx.noteBehaviorOrderFinding(order, featuring, reason)
	return false
}

func endFeature(end lower.BehaviorOrderEnd) *symbols.Symbol {
	if len(end.Path) == 0 {
		return nil
	}
	return end.Path[len(end.Path)-1]
}

func (ctx *Context) deferredBehaviorReady(behavior *ObjectBehavior) bool {
	return len(ctx.behaviorOrderBlocks(behavior)) == 0
}

func (ctx *Context) behaviorPerformanceEnded(behavior *ObjectBehavior) bool {
	if behavior == nil || behavior.Object == nil {
		return true
	}
	if life, ok := ctx.lives[behavior.Object.ID]; ok && life.destroyed {
		return true
	}
	if behavior.completed() {
		return true
	}
	var occurrence *Instance
	if behavior.Action != nil {
		occurrence = behavior.Action.occurrence
	} else if behavior.State != nil {
		occurrence = behavior.State.occurrence
	}
	if occurrence != nil {
		if life, ok := ctx.lives[occurrence.ID]; ok && life.ended != 0 {
			return true
		}
	}
	return false
}

func (ctx *Context) noteBehaviorOrderFindings(behavior *ObjectBehavior) {
	if behavior == nil || behavior.deferred == nil {
		return
	}
	for _, order := range ctx.behaviorOrdersFor(behavior.member, behavior.Object, true) {
		if order.Refusal != nil {
			continue
		}
		for _, featuring := range ctx.behaviorOrderMatches(behavior.Object, behavior.member, order, order.Later) {
			if !ctx.laterEndIsUnique(order, featuring, nil, nil) {
				continue
			}
			predecessors := ctx.behaviorOrderEndPerformances(order, order.Earlier, featuring)
			reason := ""
			switch len(predecessors) {
			case 0:
				reason = "no earlier-end performance is present; whether one is required is open under KERML-29"
			case 1:
				continue
			default:
				reason = fmt.Sprintf("%d earlier-end performances are present in one featuring instance; their pairing is open under KERML-29", len(predecessors))
			}
			ctx.noteBehaviorOrderFinding(order, featuring, reason)
		}
	}
}

func (ctx *Context) noteBehaviorOrderFinding(order lower.BehaviorOrder, featuring *Instance, reason string) {
	if featuring == nil {
		return
	}
	key := successionOrderNoteKey{order: order.Decl, object: featuring.ID}
	if ctx.successionOrderNotes[key] {
		return
	}
	ctx.successionOrderNotes[key] = true
	ctx.note(SuccessionOrdersNothing{
		Reason: fmt.Sprintf("succession %s releases %s: %s", behaviorOrderName(order), symbolText(endFeature(order.Later)), reason),
		File:   order.File,
		Span:   order.Span,
	})
}

func (ctx *Context) releaseDeferredBehavior(behavior *ObjectBehavior) error {
	if behavior == nil || behavior.deferred == nil {
		return nil
	}
	ctx.noteBehaviorOrderFindings(behavior)
	decl := *behavior.deferred
	if ctx.trace != nil {
		ctx.trace.RecordBehaviorStart(decl.behavior.Kind.String(), decl.behavior.Name, behavior.Object.ID)
	}
	ctx.attachBehavior(behavior.Object, decl.member)
	started, err := ctx.attachOneClassifierBehavior(behavior.Object, decl, behavior.performance)
	ctx.behaviorAttached(behavior.Object, decl.member)
	if started != nil {
		behavior.Symbol = started.Symbol
		behavior.bindings = started.bindings
		behavior.kinds = started.kinds
		behavior.Action = started.Action
		behavior.State = started.State
		behavior.Err = started.Err
	}
	behavior.deferred = nil
	if err != nil {
		if started == nil || !errors.Is(err, ErrUnboundParameter) {
			if started != nil {
				started.leaveClock()
			}
			return err
		}
		ctx.endFailedPerformance(behavior, fmt.Errorf("%s: %w", behavior.Describe(), err))
	}
	ctx.workChanged()
	return nil
}

// noneHeldMemo remembers a cycle check that found no behavior held; it answers
// until a behavior enters held, the one move that can put a cycle in the graph.
type noneHeldMemo struct {
	taken bool
	held  uint64
}

func (m noneHeldMemo) answers(ctx *Context) bool { return m.taken && m.held == ctx.held }

// behaviorHeld records a behavior entering held, retiring the none-held memo.
func (ctx *Context) behaviorHeld() { ctx.held++ }

// successionCycle is walkSuccessionCycle, answered from the memo while no behavior
// is held: the walk visits held behaviors only, so with none it finds no cycle.
func (ctx *Context) successionCycle() error {
	if ctx.noneHeld.answers(ctx) {
		return nil
	}
	for _, behavior := range ctx.objectBehaviors {
		if behavior.deferred != nil {
			return ctx.walkSuccessionCycle()
		}
	}
	ctx.noneHeld = noneHeldMemo{taken: true, held: ctx.held}
	return nil
}

func (ctx *Context) walkSuccessionCycle() error {
	var visit func(*ObjectBehavior) bool
	state := make(map[*ObjectBehavior]uint8)
	stack := make([]*ObjectBehavior, 0)
	var cycle []*ObjectBehavior
	visit = func(current *ObjectBehavior) bool {
		switch state[current] {
		case 1:
			at := slices.Index(stack, current)
			cycle = append(cycle, stack[at:]...)
			cycle = append(cycle, current)
			return true
		case 2:
			return false
		}
		state[current] = 1
		stack = append(stack, current)
		for _, block := range ctx.behaviorOrderBlocks(current) {
			if block.predecessor.deferred != nil && visit(block.predecessor) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[current] = 2
		return false
	}
	for _, behavior := range ctx.objectBehaviors {
		if behavior.deferred != nil && visit(behavior) {
			names := make([]string, 0, len(cycle))
			for _, item := range cycle {
				names = append(names, item.Describe())
			}
			return &SuccessionOrderError{
				Code:    successionOrderCycleCode,
				Message: "succession-order-cycle: held behaviors wait on each other: " + strings.Join(names, " -> "),
				Err:     ErrSuccessionOrderCycle,
			}
		}
	}
	return nil
}

func (ctx *Context) checkSuccessionOrderViolation(inst *Instance, member *symbols.Symbol) error {
	if inst == nil || member == nil {
		return nil
	}
	for _, order := range ctx.behaviorOrdersFor(member, inst, true) {
		if order.Refusal != nil {
			continue
		}
		for _, featuring := range ctx.behaviorOrderMatches(inst, member, order, order.Later) {
			if !ctx.laterEndIsUnique(order, featuring, inst, member) {
				continue
			}
			targets := ctx.orderEndTargetsForOrder(order, order.Earlier, featuring)
			for _, candidate := range ctx.objectBehaviors {
				if candidate == nil || candidate.member == nil ||
					!ctx.behaviorOrderFeatureMatches(candidate.Object, candidate.member, endFeature(order.Earlier)) ||
					!slices.Contains(targets, candidate.Object) ||
					!ctx.behaviorPerformanceRunning(candidate) {
					continue
				}
				return ctx.successionViolation(order, candidate, inst, featuring)
			}
		}
	}
	for _, order := range ctx.behaviorOrdersFor(member, inst, false) {
		if order.Refusal != nil {
			continue
		}
		for _, featuring := range ctx.behaviorOrderMatches(inst, member, order, order.Earlier) {
			laterCount := ctx.behaviorOrderEndPerformanceCount(order, order.Later, featuring, nil, nil)
			if laterCount == 0 {
				continue
			}
			if laterCount > 1 {
				ctx.laterEndIsUnique(order, featuring, nil, nil)
				continue
			}
			targets := ctx.orderEndTargetsForOrder(order, order.Later, featuring)
			for _, candidate := range ctx.objectBehaviors {
				if candidate == nil || candidate.member == nil ||
					!ctx.behaviorOrderFeatureMatches(candidate.Object, candidate.member, endFeature(order.Later)) ||
					!slices.Contains(targets, candidate.Object) ||
					!ctx.behaviorPerformanceBegun(candidate) {
					continue
				}
				return ctx.successionViolation(order, candidate, inst, featuring)
			}
		}
	}
	return nil
}

func (ctx *Context) behaviorPerformanceBegun(behavior *ObjectBehavior) bool {
	life, ok := ctx.behaviorPerformanceLife(behavior)
	return ok && life.began != 0
}

func (ctx *Context) behaviorPerformanceRunning(behavior *ObjectBehavior) bool {
	life, ok := ctx.behaviorPerformanceLife(behavior)
	return ok && life.began != 0 && life.ended == 0
}

func (ctx *Context) behaviorPerformanceLife(behavior *ObjectBehavior) (life, bool) {
	if behavior == nil || behavior.deferred != nil {
		return life{}, false
	}
	var occurrence *Instance
	if behavior.Action != nil {
		occurrence = behavior.Action.occurrence
	} else if behavior.State != nil {
		occurrence = behavior.State.occurrence
	}
	if occurrence == nil {
		return life{}, false
	}
	performance, ok := ctx.lives[occurrence.ID]
	return performance, ok
}

func (ctx *Context) successionViolation(order lower.BehaviorOrder, other *ObjectBehavior, inst, featuring *Instance) error {
	earlier := endFeature(order.Earlier)
	later := endFeature(order.Later)
	earlierObject, laterObject := inst.ID, other.Object.ID
	activeFeature := later
	if ctx.behaviorOrderFeatureMatches(other.Object, other.member, earlier) {
		earlierObject, laterObject = other.Object.ID, inst.ID
		activeFeature = earlier
	}
	return &SuccessionOrderError{
		Code: successionOrderViolatedCode,
		Message: fmt.Sprintf("succession-order-violated: succession %s orders %s of object #%d before %s of object #%d, but %s of object #%d has already begun in featuring object #%d",
			behaviorOrderName(order), symbolText(earlier), earlierObject,
			symbolText(later), laterObject, symbolText(activeFeature), other.Object.ID, featuring.ID),
		Err: ErrSuccessionOrderViolated,
	}
}

func behaviorOrderName(order lower.BehaviorOrder) string {
	if order.Name != "" {
		return order.Name
	}
	return "first-then succession"
}

func (ctx *Context) noteRefusedBehaviorOrders(inst *Instance) {
	if inst == nil {
		return
	}
	for _, order := range ctx.behaviorOrders() {
		if order.Refusal == nil ||
			!lower.BehaviorOrderHasExecutableEnd(ctx.model.semantics, order) {
			continue
		}
		if !ctx.instanceInBehaviorOrder(inst, order) {
			continue
		}
		key := successionOrderNoteKey{order: order.Decl, object: inst.ID}
		if ctx.successionOrderNotes[key] {
			continue
		}
		ctx.successionOrderNotes[key] = true
		ctx.note(SuccessionOrdersNothing{
			Reason: order.Refusal.Reason,
			File:   order.File,
			Span:   order.Refusal.Span,
		})
	}
}

func (ctx *Context) instanceInBehaviorOrder(inst *Instance, order lower.BehaviorOrder) bool {
	if order.Featuring != nil {
		if symbolIsUsage(order.Featuring) {
			instances, _ := ctx.liveOccurrences(order.Featuring)
			if slices.Contains(instances, inst) {
				return true
			}
		} else if ctx.instanceConforms(inst, order.Featuring) {
			return true
		}
		return false
	}
	for _, end := range []lower.BehaviorOrderEnd{order.Earlier, order.Later} {
		if len(end.Path) < 2 {
			continue
		}
		heads, _ := ctx.liveOccurrences(end.Path[0])
		targets := heads
		if len(end.Path) > 2 {
			targets = ctx.walkOrderFeaturePath(heads, end.Path[1:len(end.Path)-1])
		}
		if slices.Contains(targets, inst) {
			return true
		}
	}
	return false
}

func symbolIsUsage(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind.IsFeature() && !sym.Kind.IsDefinition()
}
