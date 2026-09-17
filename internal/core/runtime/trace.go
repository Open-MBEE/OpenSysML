package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// traceIndent is one nesting level of evaluation depth in a recorded trace.
const traceIndent = "  "

// TraceKind classifies a trace record.
type TraceKind int

const (
	// TraceLine is a record the trace prints as a line and a query does not read
	// past its text: steps, evaluations, calculations, object lifecycle.
	TraceLine TraceKind = iota
	// TraceAccept is an event a behavior took off its queue: a signal or an
	// operation call. The trace prints no line for it.
	TraceAccept
	// TraceSend is a message posted onto the bus. The trace prints no line for it.
	TraceSend
	// TraceTransition is a transition fired.
	TraceTransition
	// TraceEntry is a state entered.
	TraceEntry
	// TraceExit is a state exited.
	TraceExit
	// TraceDo is one step of a state's do behavior.
	TraceDo
	// TraceChoice is a choice point the run drew.
	TraceChoice
	// TraceGuard is a guard the run read to report a choice and could not evaluate.
	TraceGuard
)

// String names the kind as a query reads it.
func (k TraceKind) String() string {
	switch k {
	case TraceLine:
		return "line"
	case TraceAccept:
		return "accept"
	case TraceSend:
		return "send"
	case TraceTransition:
		return "transition"
	case TraceEntry:
		return "entry"
	case TraceExit:
		return "exit"
	case TraceDo:
		return "do"
	case TraceChoice:
		return "choice"
	case TraceGuard:
		return "guard"
	}
	return fmt.Sprintf("TraceKind(%d)", int(k))
}

// TraceOrigin is where a record was made: the clock's instant, the object whose
// behavior made it and that behavior, each nil where the run has none.
type TraceOrigin struct {
	At       float64
	Object   *Instance
	Behavior *symbols.Symbol
}

// TraceRecord is one entry of a run's trace. The printed line is derived from
// the record, so what a query reads and what `-trace` prints cannot drift.
type TraceRecord struct {
	Kind   TraceKind
	Origin TraceOrigin
	// State is the state entered, exited or stepped; From and To are a fired
	// transition's endpoints.
	State, From, To string
	// Event is the trigger a transition fired on, or the signal or operation an
	// accept or send carries; Payload is the message's payload.
	Event   string
	Payload map[string]Value
	// Target is the object a send was addressed to, nil for a broadcast or a
	// destination named only as text (kept in To).
	Target *Instance
	// Action reports an entry or exit behavior ran with the entry or exit.
	Action bool
	// Note is a TraceChoice's or TraceGuard's note.
	Note RunNote
	// text and depth are a TraceLine's line and nesting.
	text  string
	depth int
}

// Line is the line the trace prints for the record; printed is false for a
// record the trace keeps without printing.
func (r TraceRecord) Line() (line string, printed bool) {
	switch r.Kind {
	case TraceLine:
		return strings.Repeat(traceIndent, r.depth) + r.text, true
	case TraceAccept, TraceSend:
		return "", false
	case TraceTransition:
		if r.Event == "" {
			return fmt.Sprintf("transition: %s -> %s", r.From, r.To), true
		}
		return fmt.Sprintf("transition: %s -> %s (event: %s)", r.From, r.To, r.Event), true
	case TraceEntry:
		if r.Action {
			return fmt.Sprintf("enter: %s (entry action)", r.State), true
		}
		return fmt.Sprintf("enter: %s", r.State), true
	case TraceExit:
		if r.Action {
			return fmt.Sprintf("exit: %s (exit action)", r.State), true
		}
		return fmt.Sprintf("exit: %s", r.State), true
	case TraceDo:
		return fmt.Sprintf("do: %s", r.State), true
	case TraceChoice, TraceGuard:
		return r.Note.String(), true
	}
	return "", false
}

// Text is what a query reads as the record's text: its printed line, or for an
// accept or send its kind and event.
func (r TraceRecord) Text() string {
	if line, printed := r.Line(); printed {
		return line
	}
	return r.Kind.String() + " " + r.Event
}

// TraceRecorder keeps a run's trace as typed records, in the order they were
// made, and prints them as the deterministic lines the golden trace tests read.
//
// Evaluation entries are recorded in post-order: the sub-expressions of an
// expression appear before it, indented one level deeper, so sibling evaluation
// order and nesting are both readable off the trace. A constant sub-expression
// is answered by the semantic constant folder without evaluating its operands,
// so it appears with no children.
//
// Clear marks records printed rather than dropping them: a query reads the whole
// run, or the most recent limit records of one bounded by NewEventRecorder.
type TraceRecorder struct {
	records []TraceRecord
	printed int
	enabled bool
	depth   int
	// queryOnly keeps no TraceLine: nothing prints the recorder, queries read it.
	queryOnly bool
	// limit is the most records kept, 0 for all; dropped and horizon count and date the oldest discarded.
	limit   int
	dropped int
	horizon float64
}

// NewTraceRecorder creates a new trace recorder.
func NewTraceRecorder() *TraceRecorder {
	return &TraceRecorder{
		records: make([]TraceRecord, 0),
		enabled: true,
	}
}

// NewEventRecorder creates a recorder nothing prints, keeping the most recent limit records (all when 0).
func NewEventRecorder(limit int) *TraceRecorder {
	return &TraceRecorder{
		records:   make([]TraceRecord, 0),
		enabled:   true,
		queryOnly: true,
		limit:     max(limit, 0),
	}
}

// Enable enables trace recording.
func (tr *TraceRecorder) Enable() {
	tr.enabled = true
}

// Disable disables trace recording.
func (tr *TraceRecorder) Disable() {
	tr.enabled = false
}

// add keeps one record.
func (tr *TraceRecorder) add(r TraceRecord) {
	if !tr.enabled || (tr.queryOnly && r.Kind == TraceLine) {
		return
	}
	tr.records = append(tr.records, r)
	tr.trim()
}

// trim drops the oldest records past the limit, remembering how many and up to when.
func (tr *TraceRecorder) trim() {
	if tr.limit == 0 || len(tr.records) <= tr.limit {
		return
	}
	n := len(tr.records) - tr.limit
	tr.dropped += n
	tr.horizon = tr.records[n-1].Origin.At
	tr.printed = max(tr.printed-n, 0)
	tr.records = tr.records[n:]
}

// Dropped reports how many records the limit discarded and the instant of the last of them.
func (tr *TraceRecorder) Dropped() (count int, upTo float64) {
	return tr.dropped, tr.horizon
}

// line keeps a TraceLine at nesting depth 0.
func (tr *TraceRecorder) line(text string) {
	tr.add(TraceRecord{Kind: TraceLine, text: text})
}

// RecordActionStep records an action executor step with active tokens.
// Tokens are sorted by ID for deterministic output.
func (tr *TraceRecorder) RecordActionStep(step int, tokens []Token) {
	if !tr.enabled {
		return
	}

	if len(tokens) == 0 {
		tr.line(fmt.Sprintf("step %d: no active tokens", step))
		return
	}

	// Sort tokens by ID for determinism
	sorted := make([]Token, len(tokens))
	copy(sorted, tokens)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	// Format: step N: token T1@node1, token T2@node2
	var parts []string
	for _, t := range sorted {
		nodeName := nodeIdentifier(t.Location)
		parts = append(parts, fmt.Sprintf("token %d@%s", t.ID, nodeName))
	}

	tr.line(fmt.Sprintf("step %d: %s", step, strings.Join(parts, ", ")))
}

// RecordNote records a run's note where it was made: before the step line of the
// action step it belongs to, or before the transition it decided.
func (tr *TraceRecorder) RecordNote(origin TraceOrigin, n RunNote) {
	kind := TraceChoice
	if _, isGuard := n.(UnevaluableGuard); isGuard {
		kind = TraceGuard
	}
	tr.add(TraceRecord{Kind: kind, Origin: origin, Note: n})
}

// Mark is the whole-run position the next record takes, for RecordAcceptAt to insert there later.
func (tr *TraceRecorder) Mark() int {
	return tr.dropped + len(tr.records)
}

// RecordAcceptAt records an accept as RecordAccept does, placed at mark: before
// the records the dispatch of the event made. A mark already printed stays printed.
func (tr *TraceRecorder) RecordAcceptAt(mark int, origin TraceOrigin, event string, payload map[string]Value) {
	if !tr.enabled {
		return
	}
	record := TraceRecord{Kind: TraceAccept, Origin: origin, Event: event, Payload: payload}
	mark = min(max(mark-tr.dropped, 0), len(tr.records))
	tr.records = append(tr.records, TraceRecord{})
	copy(tr.records[mark+1:], tr.records[mark:])
	tr.records[mark] = record
	if mark < tr.printed {
		tr.printed++
	}
	tr.trim()
}

// RecordStateTransition records a transition fired, with the trigger it fired on.
func (tr *TraceRecorder) RecordStateTransition(origin TraceOrigin, fromState, toState string, event string) {
	tr.add(TraceRecord{Kind: TraceTransition, Origin: origin, From: fromState, To: toState, Event: event})
}

// RecordAccept records an event a behavior took off its queue: the signal or
// operation it names, with the payload it carries.
func (tr *TraceRecorder) RecordAccept(origin TraceOrigin, event string, payload map[string]Value) {
	tr.add(TraceRecord{Kind: TraceAccept, Origin: origin, Event: event, Payload: payload})
}

// RecordSend records a message posted onto the bus by the object at origin, or
// from outside the run where it has none.
func (tr *TraceRecorder) RecordSend(origin TraceOrigin, msg Message, target *Instance) {
	event := msg.SignalType
	if msg.EventName != "" {
		event = msg.EventName
	}
	tr.add(TraceRecord{Kind: TraceSend, Origin: origin, Event: event, To: msg.Target, Target: target, Payload: msg.Payload})
}

// RecordStateTerminate records the machine's performance ending at the terminate
// action stop, with the states whose do behaviors it abandoned.
func (tr *TraceRecorder) RecordStateTerminate(stop string, abandoned []string) {
	if len(abandoned) == 0 {
		tr.line(fmt.Sprintf("terminate: %s", stop))
		return
	}
	tr.line(fmt.Sprintf("terminate: %s (do behavior abandoned: %s)", stop, strings.Join(abandoned, ", ")))
}

// RecordStateEndedWithOccurrence records the machine's performance ending with the
// occurrence a `terminate` named, with the states whose do behaviors it abandoned.
func (tr *TraceRecorder) RecordStateEndedWithOccurrence(machine string, abandoned []string) {
	if len(abandoned) == 0 {
		tr.line(fmt.Sprintf("terminated with occurrence: %s", machine))
		return
	}
	tr.line(fmt.Sprintf("terminated with occurrence: %s (do behavior abandoned: %s)", machine, strings.Join(abandoned, ", ")))
}

// RecordStateEntry records entering a state with optional entry action execution.
func (tr *TraceRecorder) RecordStateEntry(origin TraceOrigin, state string, hasEntryAction bool) {
	tr.add(TraceRecord{Kind: TraceEntry, Origin: origin, State: state, Action: hasEntryAction})
}

// RecordStateExit records exiting a state with optional exit action execution.
func (tr *TraceRecorder) RecordStateExit(origin TraceOrigin, state string, hasExitAction bool) {
	tr.add(TraceRecord{Kind: TraceExit, Origin: origin, State: state, Action: hasExitAction})
}

// RecordActionNodeEnter records a token entering the flow an action node owns,
// whose steps are that node's subperformances.
func (tr *TraceRecorder) RecordActionNodeEnter(node string) {
	tr.line(fmt.Sprintf("enter action node: %s", node))
}

// RecordActionNodeExit records the flow an action node owns having completed,
// which is when the node itself completes.
func (tr *TraceRecorder) RecordActionNodeExit(node string) {
	tr.line(fmt.Sprintf("leave action node: %s", node))
}

// RecordActionTerminate records a performance ended by a terminate, with the tokens
// dropped from its flow in the order they were, lowest ID first.
func (tr *TraceRecorder) RecordActionTerminate(perf string, dropped []Token) {
	if !tr.enabled {
		return
	}
	if len(dropped) == 0 {
		tr.line(fmt.Sprintf("terminate %s: no token dropped", perf))
		return
	}
	parts := make([]string, 0, len(dropped))
	for _, t := range dropped {
		parts = append(parts, fmt.Sprintf("token %d@%s", t.ID, nodeIdentifier(t.Location)))
	}
	tr.line(fmt.Sprintf("terminate %s: dropped %s", perf, strings.Join(parts, ", ")))
}

// RecordActionTerminatePending records a performance a terminate ended at a parked token:
// one waiting at an accept, or one whose step had yet to begin.
func (tr *TraceRecorder) RecordActionTerminatePending(perf string, waiting bool) {
	how := "ended before it began"
	if waiting {
		how = "ended waiting"
	}
	tr.line(fmt.Sprintf("terminate %s: %s", perf, how))
}

// RecordCalcEnter records entering a calc invocation and opens a nesting level.
func (tr *TraceRecorder) RecordCalcEnter(name string) {
	tr.RecordCalculationEnter("calc", name)
}

// RecordCalculationEnter records entering a calculation of the given kind — a
// calc, or an analysis case — and opens a nesting level.
func (tr *TraceRecorder) RecordCalculationEnter(kind, name string) {
	tr.record(fmt.Sprintf("enter %s %s", kind, name))
	tr.depth++
}

// RecordCalcBind records binding one calc input parameter. source names where
// the value came from ("argument" or "default").
func (tr *TraceRecorder) RecordCalcBind(param string, value Value, source string) {
	tr.record(fmt.Sprintf("bind %s = %s [%s]", param, FormatTraceValue(value), source))
}

// RecordStatement records one body statement about to run and opens a nesting
// level for the expressions it evaluates and the statements it contains.
func (tr *TraceRecorder) RecordStatement(label string) {
	tr.record("stmt " + label)
	tr.depth++
}

// RecordLoopIteration records one iteration of a loop and opens a nesting level
// for what that iteration does, which is how a loop's progress is readable off
// the trace.
func (tr *TraceRecorder) RecordLoopIteration(iteration int) {
	tr.record(fmt.Sprintf("iteration %d", iteration))
	tr.depth++
}

// EndStatement closes the level RecordStatement or RecordLoopIteration opened.
func (tr *TraceRecorder) EndStatement() {
	tr.closeLevel()
}

// RecordCalcExit closes a calc invocation's nesting level and records its result.
func (tr *TraceRecorder) RecordCalcExit(name string, result Value) {
	tr.RecordCalculationExit("calc", name, result)
}

// RecordCalculationExit closes a calculation's nesting level and records its result.
func (tr *TraceRecorder) RecordCalculationExit(kind, name string, result Value) {
	tr.closeLevel()
	tr.record(fmt.Sprintf("exit %s %s -> %s", kind, name, FormatTraceValue(result)))
}

// RecordCalcExitError closes a calc invocation that failed, recording why.
// The failure is part of the ordering contract: it says how far binding and
// evaluation got before the calc gave up.
func (tr *TraceRecorder) RecordCalcExitError(name string, err error) {
	tr.RecordCalculationExitError("calc", name, err)
}

// RecordCalculationExitError closes a calculation that failed, recording why.
func (tr *TraceRecorder) RecordCalculationExitError(kind, name string, err error) {
	tr.closeLevel()
	tr.record(fmt.Sprintf("exit %s %s -> error: %v", kind, name, err))
}

// BeginEval opens a nesting level for one expression's sub-expressions.
func (tr *TraceRecorder) BeginEval() {
	tr.depth++
}

// EndEval closes the level BeginEval opened and records the expression's
// outcome, so the entry appears after the sub-expressions it consumed.
func (tr *TraceRecorder) EndEval(label string, value Value, err error) {
	if tr.depth > 0 {
		tr.depth--
	}
	if err != nil {
		tr.record(fmt.Sprintf("eval %s -> error: %v", label, err))
		return
	}
	tr.record(fmt.Sprintf("eval %s -> %s", label, FormatTraceValue(value)))
}

// nesting is the depth the next entry is recorded at, 0 for no recorder.
func (tr *TraceRecorder) nesting() int {
	if tr == nil {
		return 0
	}
	return tr.depth
}

// setNesting sets the depth the next entry is recorded at: work pausing mid-entry
// closes the levels it holds open while other work records, reopening them when resumed.
func (tr *TraceRecorder) setNesting(depth int) {
	if tr != nil {
		tr.depth = max(depth, 0)
	}
}

// closeLevel closes the innermost nesting level an entry opened.
func (tr *TraceRecorder) closeLevel() {
	if tr.depth > 0 {
		tr.depth--
	}
}

// record appends one entry at the current nesting depth. Depth is tracked
// whether or not recording is enabled, so nesting stays consistent across a
// recorder that is disabled and re-enabled mid-evaluation.
func (tr *TraceRecorder) record(entry string) {
	tr.add(TraceRecord{Kind: TraceLine, text: entry, depth: tr.depth})
}

// RecordDoStep records one action of a state's do behavior, which is how the
// interleaving of concurrently active states' do behaviors becomes visible.
func (tr *TraceRecorder) RecordDoStep(origin TraceOrigin, state string) {
	tr.add(TraceRecord{Kind: TraceDo, Origin: origin, State: state})
}

// RecordEvent records an event being processed.
func (tr *TraceRecorder) RecordEvent(event string, time float64) {
	tr.line(fmt.Sprintf("event: %s (t=%.1f)", event, time))
}

// RecordStateSpaceStep records the state a dynamics holds at an instant, with the
// output it computes there: one `(t, x)` sample of the run.
func (tr *TraceRecorder) RecordStateSpaceStep(action string, time float64, state, output Value) {
	if !tr.enabled {
		return
	}
	tr.line(fmt.Sprintf("state: %s t=%s x=%s y=%s",
		action, semantics.FormatReal(time), FormatValue(state), FormatValue(output)))
}

// RecordObjectMaterialized records an object being materialized, before any
// behavior of it starts.
func (tr *TraceRecorder) RecordObjectMaterialized(typeName string, id int64) {
	tr.line(fmt.Sprintf("materialize: %s #%d", typeName, id))
}

// RecordOccurrenceCreated records `create` starting an object during a call.
func (tr *TraceRecorder) RecordOccurrenceCreated(typeName string, id int64) {
	tr.line(fmt.Sprintf("create: %s #%d", typeName, id))
}

// RecordOccurrenceDestroyed records an object ending by `destroy`.
func (tr *TraceRecorder) RecordOccurrenceDestroyed(typeName string, id int64) {
	tr.line(fmt.Sprintf("destroy: %s #%d", typeName, id))
}

// RecordOccurrenceTerminated records an occurrence ending by `terminate`.
func (tr *TraceRecorder) RecordOccurrenceTerminated(typeName string, id int64) {
	tr.line(fmt.Sprintf("terminate: %s #%d", typeName, id))
}

// RecordBehaviorStart records an object's own execution of a behavior its type
// exhibits or performs starting.
func (tr *TraceRecorder) RecordBehaviorStart(kind, name string, id int64) {
	tr.line(fmt.Sprintf("start: %s %s of #%d", kind, name, id))
}

// RecordBehaviorRun records an object's behavior being advanced, which is how
// the interleaving of several objects' behaviors becomes visible.
func (tr *TraceRecorder) RecordBehaviorRun(kind, name string, id int64) {
	tr.line(fmt.Sprintf("run: %s %s of #%d", kind, name, id))
}

// Entries returns the lines of the records made since the last Clear, in order.
func (tr *TraceRecorder) Entries() []string {
	entries := make([]string, 0, len(tr.records)-tr.printed)
	for _, r := range tr.records[tr.printed:] {
		if line, printed := r.Line(); printed {
			entries = append(entries, line)
		}
	}
	return entries
}

// Records returns the records kept in order, printed ones included. The slice is read-only.
func (tr *TraceRecorder) Records() []TraceRecord {
	return tr.records
}

// String returns the trace as a single string (newline-separated entries).
func (tr *TraceRecorder) String() string {
	return strings.Join(tr.Entries(), "\n")
}

// Clear marks every record printed, so Entries starts over, and resets nesting depth.
func (tr *TraceRecorder) Clear() {
	tr.printed = len(tr.records)
	tr.depth = 0
}

// FormatTraceValue renders a runtime value canonically for a trace. Set
// elements are sorted by their rendering, since a set has no order of its own
// and its backing map does not iterate in a stable one.
func FormatTraceValue(v Value) string {
	switch v.Kind {
	case ValConst:
		return formatConst(v.Const)
	case ValNull:
		return "null"
	case ValString:
		return strconv.Quote(v.Str())
	case ValInstance:
		return fmt.Sprintf("instance#%d", v.Instance)
	case ValSequence:
		if v.Sequence() == nil {
			return "()"
		}
		parts := make([]string, 0, v.Sequence().Size())
		for _, elem := range v.Sequence().Elements() {
			parts = append(parts, FormatTraceValue(elem))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case ValSet:
		if v.Set() == nil {
			return "{}"
		}
		parts := make([]string, 0, v.Set().Size())
		for _, elem := range v.Set().Elements() {
			parts = append(parts, FormatTraceValue(elem))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case ValQuantity:
		if v.Quantity() == nil {
			return v.Kind.String()
		}
		// A unit-carrying value is rendered as the REPL renders it, with the
		// magnitude in the trace's own convention for numbers.
		return v.Quantity().TextWithMagnitude(formatConst(v.Quantity().Num))
	case ValVariant:
		if v.Variant() == nil {
			return v.Kind.String()
		}
		return v.Variant().Name
	case ValEnumLiteral:
		return v.LiteralText()
	case ValComplex:
		return FormatComplex(v.Complex())
	case ValArray:
		if v.Array() == nil {
			return v.Kind.String()
		}
		return v.Array().Format(FormatTraceValue)
	case ValVector:
		if v.Vector() == nil {
			return v.Kind.String()
		}
		return v.Vector().format(formatConst)
	case ValVectorQuantity:
		if v.VectorQuantity() == nil {
			return v.Kind.String()
		}
		return v.VectorQuantity().format(formatConst)
	case ValTensorQuantity:
		if v.TensorQuantity() == nil {
			return v.Kind.String()
		}
		return v.TensorQuantity().format(formatConst)
	case ValMeasurementRef:
		if v.MeasurementRef() == nil {
			return v.Kind.String()
		}
		return v.MeasurementRef().String()
	case ValCoordinateFrame:
		if v.CoordinateFrame() == nil {
			return v.Kind.String()
		}
		return v.CoordinateFrame().String()
	case ValCoordinateTransformation:
		if v.CoordinateTransformation() == nil {
			return v.Kind.String()
		}
		return v.CoordinateTransformation().String()
	case ValExpr:
		return fmt.Sprintf("expr(%s)", TraceLabel(v.Expr()))
	case ValFunction:
		return fmt.Sprintf("calc(%s)", v.FunctionName())
	case ValMetaobject:
		return v.MetaobjectText()
	case ValUndetermined:
		return fmt.Sprintf("undetermined(%s)", v.Undetermined().Reason())
	default:
		return v.Kind.String()
	}
}

// formatConst renders a folded constant. Reals print with the shortest form
// that round-trips, so the same value always renders the same way.
func formatConst(c semantics.Value) string {
	switch c.Kind {
	case semantics.ValInt:
		return strconv.FormatInt(c.Int, 10)
	case semantics.ValReal:
		return semantics.FormatReal(c.Real)
	case semantics.ValBool:
		return strconv.FormatBool(c.Bool)
	case semantics.ValInfinity:
		return "*"
	default:
		return "invalid"
	}
}

// literalLabel prefixes the trace label of a literal expression.
const literalLabel = "literal "

// TraceLabel names an expression node for a trace: its kind plus the token that
// identifies it, which is stable across reformatting of the source.
func TraceLabel(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return "nil"
	case *ast.LiteralInteger:
		return literalLabel + n.Value
	case *ast.LiteralReal:
		return literalLabel + n.Value
	case *ast.LiteralBool:
		return literalLabel + strconv.FormatBool(n.Value)
	case *ast.LiteralString:
		return literalLabel + n.Value
	case *ast.LiteralInfinity:
		return "literal *"
	case *ast.NullExpr:
		return "null"
	case *ast.FeatureReference:
		return "feature " + qualifiedNameToString(n.Name)
	case *ast.QualifiedName:
		return "feature " + qualifiedNameToString(n)
	case *ast.FeatureChainExpr:
		return "chain " + qualifiedNameToString(n.Member)
	case *ast.OperatorExpr:
		return "operator " + n.Operator.String()
	case *ast.InvocationExpr:
		return "invoke " + qualifiedNameToString(n.Type)
	case *ast.SequenceExpr:
		return fmt.Sprintf("sequence of %d", len(n.Elements))
	case *ast.CollectExpr:
		return "collect"
	case *ast.SelectExpr:
		return "select"
	case *ast.IndexExpr:
		return "index"
	case *ast.BodyExpr:
		return "body"
	case *ast.ConstructorExpr:
		return "construct " + qualifiedNameToString(n.Type)
	case *ast.MetadataAccessExpr:
		return "metadata"
	default:
		return fmt.Sprintf("%T", node)
	}
}

// nodeIdentifier returns a stable identifier for an AST node.
// Prefers named nodes (Ident.Name), falls back to node type.
func nodeIdentifier(node ast.Node) string {
	if node == nil {
		return "nil"
	}

	switch n := node.(type) {
	case *ast.Usage:
		// A step written as a redefinition is traced under the name it
		// answers to, the one it redefines.
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return fmt.Sprintf("usage_%s", n.Kind)
	case *ast.Definition:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return fmt.Sprintf("def_%s", n.Kind)
	case *ast.StateNode:
		if n.Name != "" {
			return n.Name
		}
		return "state_anonymous"
	case *ast.InitialNode:
		return controlNodeName(n.Name(), "initial")
	case *ast.FinalNode:
		return "done"
	case *ast.ForkNode:
		return controlNodeName(n.Name, "fork")
	case *ast.JoinNode:
		return controlNodeName(n.Name, "join")
	case *ast.MergeNode:
		return controlNodeName(n.Name, "merge")
	case *ast.DecisionNode:
		return controlNodeName(n.Name, "decision")
	case *ast.ActionExecutionNode:
		return controlNodeName(n.Name, "action")
	default:
		return fmt.Sprintf("%T", node)
	}
}

// controlNodeName names an unnamed control node by what it does, since a Go
// type name means nothing to someone reading a trace.
func controlNodeName(name, kind string) string {
	if name != "" {
		return name
	}
	return kind
}

// RecordCalcUsageExit closes the nesting level of a calc usage's evaluation,
// which computes the usage's output features rather than one result, so there is
// no single value to record for it.
func (tr *TraceRecorder) RecordCalcUsageExit(kind, name string) {
	tr.closeLevel()
	tr.record(fmt.Sprintf("exit %s %s", kind, name))
}

// RecordCalcUsageReuse records a calc usage read again with the inputs it
// already ran over, whose values come from that one run. It opens no nesting
// level of its own, since nothing runs.
func (tr *TraceRecorder) RecordCalcUsageReuse(kind, name string) {
	tr.record(fmt.Sprintf("reuse %s %s", kind, name))
}

// RecordCalcOutput records the value one output feature of a calc usage took.
// The outputs appear after the one evaluation of the usage's body they are read
// from, which is how the trace shows that reading several of them ran it once.
func (tr *TraceRecorder) RecordCalcOutput(calc, output string, value Value) {
	tr.record(fmt.Sprintf("output %s.%s = %s", calc, output, FormatTraceValue(value)))
}
