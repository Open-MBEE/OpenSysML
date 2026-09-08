package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The choice points an action run makes; recording one never alters what the
// executor does (reverse token order, first holding guard, later write stands).

// stepWriteKey identifies a feature one performance holds.
type stepWriteKey struct {
	holder *actionFrame
	name   string
}

// stepWrite is one write a step made: by which token, and what it wrote.
type stepWrite struct {
	token int64
	value Value
}

// beginStepWrites opens the ledger of the writes one step makes, numbered step,
// returning what to restore once the step is over.
func (e *performances) beginStepWrites(step int) func() {
	saved, savedStep := e.stepWrites, e.writeStep
	e.stepWrites, e.writeStep = make(map[stepWriteKey]stepWrite), step
	return func() { e.stepWrites, e.writeStep = saved, savedStep }
}

// beginTokenStep marks the token whose step is running, returning what to
// restore once its step is over.
func (e *performances) beginTokenStep(id int64) func() {
	saved := e.writer
	e.writer = id
	return func() { e.writer = saved }
}

// noteWrite records a write the running token made to a feature f holds; two
// tokens writing one feature different values within one step are a choice point.
func (e *performances) noteWrite(f *actionFrame, name string, value Value) {
	if e.writer == 0 || e.stepWrites == nil {
		return
	}
	key := stepWriteKey{holder: f, name: f.key(name)}
	prev, seen := e.stepWrites[key]
	e.stepWrites[key] = stepWrite{token: e.writer, value: value}
	if !seen || prev.token == e.writer {
		return
	}
	writes := []stepWrite{prev, {token: e.writer, value: value}}
	sort.Slice(writes, func(i, j int) bool { return writes[i].token < writes[j].token })
	alts := make([]string, len(writes))
	taken := 0
	for i, w := range writes {
		alts[i] = fmt.Sprintf("%s := %s by token %d", key.name, FormatTraceValue(w.value), w.token)
		if w.token == e.writer {
			taken = i
		}
	}
	file, span := e.ctx.featureLocation(f.scope, name)
	e.ctx.noteChoice(ChoicePoint{
		Kind:         ChoiceWriteOrder,
		Step:         e.writeStep,
		Alternatives: alts,
		Taken:        taken,
		File:         file,
		Span:         span,
	})
}

// featureLocation locates the declaration of the feature name resolves to in
// scope, for a diagnostic about a write to it; "" when none resolves.
func (ctx *Context) featureLocation(scope *symbols.Scope, name string) (string, source.Span) {
	if ctx.resolver == nil || scope == nil {
		return "", source.Span{}
	}
	sym, ok := ctx.resolver.LookupName(scope, name)
	if !ok || sym == nil {
		return "", source.Span{}
	}
	return sym.DocName, sym.DeclSpan
}

// stepOrder collects the tokens one step advanced that could have gone first, in
// the order it advanced them.
type stepOrder struct {
	firstNew int64          // tokens from this ID on were created by the step itself
	unready  map[int64]bool // held at a join whose branches had not all arrived
	acted    []Token
}

// beginStepOrder opens the order of a step about to run.
func (e *ActionExecutor) beginStepOrder() stepOrder {
	order := stepOrder{firstNew: e.nextTokenID, unready: make(map[int64]bool)}
	for _, t := range e.tokens {
		if consumed, held := e.arrivals(t); held && consumed == nil {
			order.unready[t.ID] = true
		}
	}
	return order
}

// stepTokenNoting steps the token at index i and notes it in order when it did
// something it could have done first (not parked, and not enabled by this step).
func (e *ActionExecutor) stepTokenNoting(i int, order *stepOrder) error {
	before := e.tokens[i]
	count := len(e.tokens)
	if err := e.stepToken(i); err != nil {
		return err
	}
	if order.eligible(before) && e.tokenActed(before, count) {
		order.acted = append(order.acted, before)
	}
	return nil
}

// eligible reports whether the token could have gone first in the step and is
// not yet noted.
func (o *stepOrder) eligible(t Token) bool {
	if t.ID >= o.firstNew || o.unready[t.ID] {
		return false
	}
	for _, a := range o.acted {
		if a.ID == t.ID {
			return false
		}
	}
	return true
}

// tokenActed reports whether the step of the token snapshotted as before moved,
// consumed, retired, resumed, forked or joined it, or took its awaited message.
func (e *ActionExecutor) tokenActed(before Token, count int) bool {
	if before.body != nil || len(e.tokens) != count {
		return true
	}
	i := e.tokenIndex(before.ID)
	if i < 0 {
		return true
	}
	after := e.tokens[i]
	return after.moved != before.moved || (before.Wait != nil && after.Wait == nil)
}

// noteTokenOrder records the tokens a step advanced as a choice point when there
// are at least two; which went first is the executor's rule, not the library's.
func (e *ActionExecutor) noteTokenOrder(step int, order stepOrder) {
	if len(order.acted) < 2 {
		return
	}
	first := order.acted[0].ID
	tokens := make([]Token, len(order.acted))
	copy(tokens, order.acted)
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].ID < tokens[j].ID })
	alts := make([]string, len(tokens))
	taken := 0
	for i, t := range tokens {
		alts[i] = fmt.Sprintf("%d@%s", t.ID, nodeIdentifier(t.Location))
		if t.ID == first {
			taken = i
		}
	}
	e.ctx.noteChoice(ChoicePoint{
		Kind:         ChoiceTokenOrder,
		Step:         step,
		Alternatives: alts,
		Taken:        taken,
		File:         e.action.DocName,
		Span:         e.action.DeclSpan,
	})
}

// noteDecisionBranches records the holding guarded successions of a decision node,
// at their declared positions, as a choice point when there are at least two.
func (e *ActionExecutor) noteDecisionBranches(frame *actionFrame, node *ast.DecisionNode, successors []lower.ActionEdge, holding []int) {
	if len(holding) < 2 {
		return
	}
	alts := make([]string, len(holding))
	for i, pos := range holding {
		alts[i] = fmt.Sprintf("%d->%s", pos+1, nodeIdentifier(successors[pos].Target))
	}
	file := e.action.DocName
	if scope := e.graphOf(frame).Scope; scope != nil && scope.DocName() != "" {
		file = scope.DocName()
	}
	e.ctx.noteChoice(ChoicePoint{
		Kind:         ChoiceDecisionBranch,
		Step:         e.stepCount + 1,
		Where:        "decision " + nodeIdentifier(node),
		Alternatives: alts,
		Taken:        0,
		File:         file,
		Span:         node.Span(),
	})
}
