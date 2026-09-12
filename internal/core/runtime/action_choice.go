package runtime

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The choice points an action run makes; recording one never alters what the
// executor does (the policy's token order and guard, later write stands).

// writeDest identifies a feature a write reaches: one a performance holds under
// its canonical name, or one an object holds under any of its names (FeatureValue).
type writeDest struct {
	holder  *actionFrame
	object  *Instance
	feature *FeatureValue
	name    string
}

// stepWrite is one write a step made: by which token, and what it wrote.
type stepWrite struct {
	token int64
	value Value
}

// destWrites is what one step wrote to one destination: each token's last write,
// in the order the tokens first wrote, and the write that stands.
type destWrites struct {
	label string // the destination as a trace names it: `x`, or `x of object #3`
	file  string
	span  source.Span
	last  []stepWrite
	stood stepWrite
}

// stepWriteLedger is what the action step under way wrote so far, by destination.
type stepWriteLedger struct {
	step   int
	writer int64 // the token whose step is running, 0 between tokens
	writes map[writeDest]*destWrites
	order  []writeDest // destinations in the order first written
}

// beginStepWrites opens the ledger of the writes one step makes, numbered step;
// the closer returned records its choices and restores the ledger it shadowed.
func (e *performances) beginStepWrites(step int) func() {
	saved := e.ctx.stepWrites
	ledger := &stepWriteLedger{step: step, writes: make(map[writeDest]*destWrites)}
	e.ctx.stepWrites = ledger
	return func() {
		ledger.noteChoices(e.ctx)
		e.ctx.stepWrites = saved
	}
}

// beginTokenStep marks the token whose step is running, returning what to
// restore once its step is over.
func (e *performances) beginTokenStep(id int64) func() {
	ledger := e.ctx.stepWrites
	if ledger == nil {
		return func() {
			// No ledger was open, so no writer was marked and none is restored.
		}
	}
	saved := ledger.writer
	ledger.writer = id
	return func() { ledger.writer = saved }
}

// noteFrameWrite records a write the running token made to a feature f holds.
func (e *performances) noteFrameWrite(f *actionFrame, name string, value Value) {
	file, span := e.ctx.featureLocation(f.scope, name)
	key := f.key(name)
	e.ctx.noteWrite(writeDest{holder: f, name: key}, key, value, file, span)
}

// noteObjectWrite records a write the running token made to a feature obj holds,
// under whichever of its names the write spelled.
func (ctx *Context) noteObjectWrite(obj *Instance, name string, value Value) {
	dest := writeDest{object: obj, name: name}
	var file string
	var span source.Span
	if fv := obj.FeatureValues[name]; fv != nil {
		dest = writeDest{object: obj, feature: fv}
		if fv.Feature != nil && fv.Feature.Symbol != nil {
			file, span = fv.Feature.Symbol.DocName, fv.Feature.Symbol.DeclSpan
		}
	}
	ctx.noteWrite(dest, fmt.Sprintf("%s of object #%d", name, obj.ID), value, file, span)
}

// noteWrite records a write the running token made to dest, labelled as a trace
// names it; destinations several tokens wrote are reported once the step ends.
func (ctx *Context) noteWrite(dest writeDest, label string, value Value, file string, span source.Span) {
	ledger := ctx.stepWrites
	if ledger == nil || ledger.writer == 0 {
		return
	}
	w := stepWrite{token: ledger.writer, value: value}
	d := ledger.writes[dest]
	if d == nil {
		d = &destWrites{label: label, file: file, span: span}
		ledger.writes[dest] = d
		ledger.order = append(ledger.order, dest)
	}
	d.stood = w
	if i := slices.IndexFunc(d.last, func(prev stepWrite) bool { return prev.token == w.token }); i >= 0 {
		d.last[i] = w
		return
	}
	d.last = append(d.last, w)
}

// noteChoices records each destination two or more tokens wrote within the step
// as a choice point, whatever they wrote: another order lets another write stand.
func (l *stepWriteLedger) noteChoices(ctx *Context) {
	for _, dest := range l.order {
		d := l.writes[dest]
		if len(d.last) < 2 {
			continue
		}
		writes := slices.Clone(d.last)
		sort.Slice(writes, func(i, j int) bool { return writes[i].token < writes[j].token })
		alts := make([]string, len(writes))
		taken := 0
		for i, w := range writes {
			alts[i] = fmt.Sprintf("%s := %s by token %d", d.label, FormatTraceValue(w.value), w.token)
			if w.token == d.stood.token {
				taken = i
			}
		}
		ctx.noteChoice(ChoicePoint{
			Kind:         ChoiceWriteOrder,
			Step:         l.step,
			Alternatives: alts,
			Taken:        taken,
			File:         d.file,
			Span:         d.span,
		})
	}
}

// featureLocation locates the declaration of the feature name resolves to in
// scope, for a diagnostic about a write to it; "" when none resolves.
func (ctx *Context) featureLocation(scope *symbols.Scope, name string) (string, source.Span) {
	if ctx.model.resolver == nil || scope == nil {
		return "", source.Span{}
	}
	sym, ok := ctx.lookupName(scope, name)
	if !ok || sym == nil {
		return "", source.Span{}
	}
	return sym.DocName, sym.DeclSpan
}

// stepOrder collects the tokens of one step that could have gone first, in the
// order the step reached them: the ones it advanced, and an accept a message in
// flight would have answered had another not taken it first.
type stepOrder struct {
	firstNew int64          // tokens from this ID on were created by the step itself
	unready  map[int64]bool // held at a join whose branches had not all arrived
	offered  map[int64]bool // at an accept a message in flight answers
	acted    []Token
}

// beginStepOrder opens the order of a step about to run.
func (e *ActionExecutor) beginStepOrder() stepOrder {
	order := stepOrder{firstNew: e.nextTokenID, unready: make(map[int64]bool), offered: make(map[int64]bool)}
	for _, t := range e.tokens {
		if consumed, held := e.arrivals(t); held && consumed == nil {
			order.unready[t.ID] = true
		}
	}
	if pending := e.ctx.acceptable(); len(pending) > 0 {
		// Matching may materialize a port; as a probe, the scan leaves the run as it was.
		defer e.ctx.beginProbe()()
		for _, t := range e.tokens {
			if e.offeredMessage(t, pending) {
				order.offered[t.ID] = true
			}
		}
	}
	return order
}

// offeredMessage reports whether one of the messages in flight answers the accept
// the token sits at, so that stepping it first would have taken the message.
func (e *ActionExecutor) offeredMessage(t Token, pending []Message) bool {
	accept, ok := e.messageAccept(t)
	if !ok {
		return false
	}
	matches, _ := e.acceptMatch(t.frame, accept, t.Location.(*ast.Usage))
	return slices.ContainsFunc(pending, matches)
}

// messageAccept returns the accept the token sits at when it is one a message,
// not a trigger, answers.
func (e *ActionExecutor) messageAccept(t Token) (lower.Accept, bool) {
	usage, ok := t.Location.(*ast.Usage)
	if !ok || t.body != nil {
		return lower.Accept{}, false
	}
	accept, isAccept := e.graphOf(t.frame).Accepts[usage]
	return accept, isAccept && accept.Trigger == nil
}

// stepTokenNoting steps the token at index i and notes the order taken when it
// could have acted first; a failed step was still its turn.
func (e *ActionExecutor) stepTokenNoting(i int, order *stepOrder) (acted bool, err error) {
	before := e.tokens[i]
	count := len(e.tokens)
	err = e.stepToken(i)
	acted = err != nil || e.tokenActed(before, count)
	if order.eligible(before) && (acted || order.offered[before.ID]) {
		order.acted = append(order.acted, before)
	}
	return acted, err
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

// noteTokenOrder records the tokens a step advanced as a choice point when there are
// at least two, or the one an exploring step picked among those able to act.
func (e *ActionExecutor) noteTokenOrder(step int, order stepOrder, schedule *tokenSchedule) {
	alts, taken, chosen := schedule.Choice()
	if !chosen {
		if len(order.acted) < 2 {
			return
		}
		first := order.acted[0].ID
		tokens := make([]Token, len(order.acted))
		copy(tokens, order.acted)
		sort.Slice(tokens, func(i, j int) bool { return tokens[i].ID < tokens[j].ID })
		alts = make([]string, len(tokens))
		for i, t := range tokens {
			alts[i] = fmt.Sprintf("%d@%s", t.ID, nodeIdentifier(t.Location))
			if t.ID == first {
				taken = i
			}
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

// chooseBranch resolves which of the holding guarded successions of a decision
// node, at their declared positions, the token takes: the position in holding, and
// the choice point to note when there are at least two.
func (e *ActionExecutor) chooseBranch(frame *actionFrame, node *ast.DecisionNode, successors []lower.ActionEdge, holding []int) (*ChoicePoint, int) {
	if len(holding) < 2 {
		return nil, 0
	}
	alts := make([]string, len(holding))
	for i, pos := range holding {
		alts[i] = branchName(successors, pos)
	}
	choice := ChoicePoint{
		Kind:         ChoiceDecisionBranch,
		Step:         e.stepCount + 1,
		Where:        "decision " + nodeIdentifier(node),
		Alternatives: alts,
		File:         e.decisionFile(frame),
		Span:         node.Span(),
	}
	choice.Taken = e.ctx.scheduling().choose(choice, nil)
	return &choice, choice.Taken
}

// noteUnevaluableGuard records the guard of the succession at position pos out of
// a decision node, probed once the branch was decided, as one with no result.
func (e *ActionExecutor) noteUnevaluableGuard(frame *actionFrame, node *ast.DecisionNode, successors []lower.ActionEdge, pos int, err error) {
	e.ctx.noteUnevaluableGuard(UnevaluableGuard{
		Step:        e.stepCount + 1,
		Where:       "decision " + nodeIdentifier(node),
		Alternative: branchName(successors, pos),
		Reason:      err.Error(),
		File:        e.decisionFile(frame),
		Span:        successors[pos].Guard.Span(),
	})
}

// branchName names a decision's succession by declared position and target.
func branchName(successors []lower.ActionEdge, pos int) string {
	return fmt.Sprintf("%d->%s", pos+1, nodeIdentifier(successors[pos].Target))
}

// decisionFile is the file the flow frame performs was declared in.
func (e *ActionExecutor) decisionFile(frame *actionFrame) string {
	if scope := e.graphOf(frame).Scope; scope != nil && scope.DocName() != "" {
		return scope.DocName()
	}
	return e.action.DocName
}
