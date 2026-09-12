package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// GraphsVersion is the version of the `graphs:<v>` form this build writes. It
// moves when a field the form carries changes meaning, is removed or is renamed;
// an added field keeps the version.
const GraphsVersion = 1

// Graphs is the `graphs:1` form: the lowered graph of a subject and of every
// behavior it performs, transitively, in a JSON an external engine reads. It is
// lossless for what the lowered IR carries, and its output is byte for byte the
// same for the same model.
type Graphs struct {
	// Version is GraphsVersion, first so a reader refuses a version it does not know.
	Version int `json:"version"`
	// Subject is the qualified name of the behavior the form was exported for.
	Subject string `json:"subject"`
	// Actions are the lowered action graphs, the subject's first when it is an
	// action, then the performed ones by name.
	Actions []*ActionForm `json:"actions,omitempty"`
	// States are the lowered state graphs, the subject's first when it is a state
	// machine, then the exhibited ones by name.
	States []*StateForm `json:"states,omitempty"`
}

// ErrGraphsSubject reports a subject no lowered graph is exported for: not an
// action or a state machine, or one the lowering refuses.
var ErrGraphsSubject = errors.New("graphs: subject has no lowered graph")

// ErrGraphsOrder reports two graph elements the export cannot tell apart, which
// would make the output depend on memory layout; the lowering never produces them.
var ErrGraphsOrder = errors.New("graphs: elements of one graph cannot be ordered")

// GraphsOf exports the lowered graph of subject, an action or a state machine
// declared in model, and of every behavior it performs, at GraphsVersion.
func GraphsOf(model *runtime.Model, subject *symbols.Symbol) (*Graphs, error) {
	if model == nil || subject == nil || subject.Decl == nil {
		return nil, ErrGraphsSubject
	}
	x := &graphsExporter{
		model:    model,
		sem:      model.Semantics(),
		text:     model.Text(),
		lowered:  map[string]bool{},
		machines: map[*symbols.Symbol]bool{},
	}
	out := &Graphs{Version: GraphsVersion, Subject: symbols.FQNOf(subject)}
	if err := x.lowerSubject(out, subject); err != nil {
		return nil, err
	}
	for len(x.pending) > 0 {
		sort.Slice(x.pending, func(i, j int) bool {
			return symbols.FQNOf(x.pending[i]) < symbols.FQNOf(x.pending[j])
		})
		sym := x.pending[0]
		x.pending = x.pending[1:]
		if err := x.lowerPerformed(out, sym); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// MarshalGraphs writes the form as canonical JSON: keys in struct order,
// no indentation, one trailing newline. encoding/json writes map keys sorted and
// the form holds no map, so the bytes are a function of the model alone.
func MarshalGraphs(g *Graphs) ([]byte, error) {
	out, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("graphs: %w", err)
	}
	return append(out, '\n'), nil
}

// graphsExporter walks one subject's lowered graphs and those of the behaviors
// they perform, lowering each declaration once.
type graphsExporter struct {
	model *runtime.Model
	sem   *semantics.Model
	text  source.Lookup

	lowered  map[string]bool // qualified names already exported or queued
	pending  []*symbols.Symbol
	machines map[*symbols.Symbol]bool // queued symbols that are state machines
}

// lowerSubject exports the subject itself, refusing one that is neither an
// action nor a state machine or whose lowering fails.
func (x *graphsExporter) lowerSubject(out *Graphs, sym *symbols.Symbol) error {
	x.lowered[symbols.FQNOf(sym)] = true
	switch sym.Kind {
	case symbols.SymbolActionDef, symbols.SymbolActionUsage:
		graph, err := lower.ToActionGraph(sym.Decl, runtime.DeclScope(sym))
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrGraphsSubject, symbols.FQNOf(sym), err)
		}
		form, err := x.actionForm(sym, graph)
		if err != nil {
			return err
		}
		out.Actions = append(out.Actions, form)
	case symbols.SymbolStateDef, symbols.SymbolStateUsage:
		graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, runtime.DeclScope(sym),
			lower.NewLibraryStateTypes(x.model.Resolver()))
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrGraphsSubject, symbols.FQNOf(sym), err)
		}
		form, err := x.stateForm(sym, graph)
		if err != nil {
			return err
		}
		out.States = append(out.States, form)
	default:
		return fmt.Errorf("%w: %s is a %s", ErrGraphsSubject, symbols.FQNOf(sym), sym.Kind)
	}
	return nil
}

// lowerPerformed exports a behavior the subject performs. A lowering the runtime
// would refuse is kept as the behavior's name and the refusal.
func (x *graphsExporter) lowerPerformed(out *Graphs, sym *symbols.Symbol) error {
	if x.machines[sym] {
		graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, runtime.DeclScope(sym),
			lower.NewLibraryStateTypes(x.model.Resolver()))
		if err != nil {
			out.States = append(out.States, &StateForm{Name: symbols.FQNOf(sym), Kind: sym.Kind.String(), Error: err.Error()})
			return nil
		}
		form, err := x.stateForm(sym, graph)
		if err != nil {
			return err
		}
		out.States = append(out.States, form)
		return nil
	}
	graph, err := lower.ToActionGraph(sym.Decl, runtime.DeclScope(sym))
	if err != nil {
		out.Actions = append(out.Actions, &ActionForm{Name: symbols.FQNOf(sym), Kind: sym.Kind.String(), Error: err.Error()})
		return nil
	}
	form, err := x.actionForm(sym, graph)
	if err != nil {
		return err
	}
	out.Actions = append(out.Actions, form)
	return nil
}

// perform queues the behaviors a node names — a `perform`, a typed action usage,
// an exhibited state — once each, resolved in scope, and returns their names.
func (x *graphsExporter) perform(scope *symbols.Scope, node ast.Node) []string {
	var names []string
	for _, qn := range performedNames(node) {
		sym, ok := x.model.Resolver().ResolveQualified(scope, qn)
		if !ok || sym == nil || sym.Decl == nil {
			continue
		}
		machine := false
		switch sym.Kind {
		case symbols.SymbolActionDef, symbols.SymbolActionUsage:
		case symbols.SymbolStateDef, symbols.SymbolStateUsage:
			machine = true
		default:
			continue
		}
		fqn := symbols.FQNOf(sym)
		names = append(names, fqn)
		if x.lowered[fqn] {
			continue
		}
		x.lowered[fqn] = true
		x.machines[sym] = machine
		x.pending = append(x.pending, sym)
	}
	return names
}

// performedNames are the names a node performs or is typed by, as the runtime
// reads them: an invocation's callee, a `perform` target, a usage's typing or
// reference.
func performedNames(node ast.Node) []*ast.QualifiedName {
	switch n := node.(type) {
	case *ast.PerformActionNode:
		if inv := n.PerformedInvocation(); inv != nil {
			return []*ast.QualifiedName{inv.Type}
		}
		if qn, ok := n.ActionRef.(*ast.QualifiedName); ok {
			return []*ast.QualifiedName{qn}
		}
	case *ast.ActionExecutionNode:
		if n.ActionRef != nil {
			return []*ast.QualifiedName{n.ActionRef}
		}
	case *ast.Usage:
		if inv := n.PerformedInvocation(); inv != nil {
			return []*ast.QualifiedName{inv.Type}
		}
		var names []*ast.QualifiedName
		for _, rel := range n.Relationships {
			if rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				names = append(names, qn)
			}
		}
		return names
	}
	return nil
}

// SpanForm locates a node in the model's text: the document and the byte span.
type SpanForm struct {
	Document string `json:"document,omitempty"`
	Offset   int    `json:"offset"`
	Len      int    `json:"len"`
}

// ExprForm is an expression as the graph carries it: its source text and where
// it was written, so an engine parses the text and the host names the place.
type ExprForm struct {
	Text string   `json:"text"`
	Span SpanForm `json:"span"`
}

// expr renders a node written in scope; nil for a nil node.
func (x *graphsExporter) expr(scope *symbols.Scope, node ast.Node) *ExprForm {
	if nilNode(node) {
		return nil
	}
	span := node.Span()
	form := &ExprForm{Span: SpanForm{Document: docOf(scope), Offset: span.Offset, Len: span.Len}}
	if x.text != nil && span.Len > 0 {
		form.Text = strings.TrimSpace(x.text(form.Span.Document, span))
	}
	return form
}

// docOf is the document a scope belongs to, "" for no scope.
func docOf(scope *symbols.Scope) string {
	if scope == nil {
		return ""
	}
	return scope.DocName()
}

// scopeName names a scope by its owner's qualified name, an anonymous one by its
// document and the place of the node it belongs to, a root one by its document.
func scopeName(scope *symbols.Scope) string {
	if scope == nil {
		return ""
	}
	if owner := scope.Owner(); owner != nil {
		return symbols.FQNOf(owner)
	}
	if node := scope.Node(); !nilNode(node) {
		span := node.Span()
		return fmt.Sprintf("%s#%d+%d", scope.DocName(), span.Offset, span.Len)
	}
	return scope.DocName()
}

// vertexIDs numbers the vertices of one graph in a traversal order that is a
// function of the lowered graph alone, so a map keyed by node is written as a
// list keyed by these numbers.
type vertexIDs struct {
	ids   map[ast.Node]int
	order []ast.Node
}

func newVertexIDs() *vertexIDs {
	return &vertexIDs{ids: map[ast.Node]int{}}
}

// add numbers a vertex on first sight and returns its number; nil is -1.
func (v *vertexIDs) add(node ast.Node) int {
	if nilNode(node) {
		return -1
	}
	if id, ok := v.ids[node]; ok {
		return id
	}
	id := len(v.order)
	v.ids[node] = id
	v.order = append(v.order, node)
	return id
}

// ref is the number of a vertex already added, or -1 for nil; a vertex first
// seen here is numbered too, so every reference resolves.
func (v *vertexIDs) ref(node ast.Node) *int {
	if nilNode(node) {
		return nil
	}
	id := v.add(node)
	return &id
}

// nilNode reports a nil node, a typed nil pointer read from a map of the lowered
// graph included.
func nilNode(node ast.Node) bool {
	if node == nil {
		return true
	}
	v := reflect.ValueOf(node)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// addSorted numbers vertices reached only through a map, in an order that is a
// function of what they are: span, kind, name, then owner. Two the export cannot
// tell apart are refused rather than written in memory order.
func (v *vertexIDs) addSorted(nodes []ast.Node, owner func(ast.Node) string) error {
	var rest []ast.Node
	for _, node := range nodes {
		if _, ok := v.ids[node]; !ok && !nilNode(node) {
			rest = append(rest, node)
		}
	}
	keys := make(map[ast.Node]string, len(rest))
	for _, node := range rest {
		span := node.Span()
		keys[node] = fmt.Sprintf("%012d:%012d:%s:%s:%s", span.Offset, span.Len, vertexKind(node), vertexName(node), owner(node))
	}
	sort.SliceStable(rest, func(i, j int) bool { return keys[rest[i]] < keys[rest[j]] })
	for i := 1; i < len(rest); i++ {
		if keys[rest[i]] == keys[rest[i-1]] && rest[i] != rest[i-1] {
			return fmt.Errorf("%w: two %s vertices named %q at the same place", ErrGraphsOrder, vertexKind(rest[i]), vertexName(rest[i]))
		}
	}
	for _, node := range rest {
		v.add(node)
	}
	return nil
}

// vertexKind labels a vertex by what it is in the lowered graph.
func vertexKind(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return "start"
	case *ast.FinalNode:
		return "end"
	case *ast.ForkNode:
		return "fork"
	case *ast.JoinNode:
		return "join"
	case *ast.MergeNode:
		return "merge"
	case *ast.DecisionNode:
		return "decision"
	case *ast.ActionExecutionNode:
		return "action"
	case *ast.PerformActionNode:
		return "perform"
	case *ast.AssignmentActionNode:
		return "assignment"
	case *ast.WhileLoopActionNode:
		return "loop"
	case *ast.IfActionNode:
		return "if"
	case *ast.SendStatement:
		return "send"
	case *ast.TerminateStatement:
		return "terminate"
	case *ast.AcceptActionUsage:
		return "accept"
	case *ast.StateNode:
		return "state"
	case *ast.PseudostateNode:
		return "pseudostate " + n.Kind.String()
	case *ast.StateRegion:
		return "region"
	case *ast.Usage:
		return n.Kind.String() + " usage"
	case *ast.Definition:
		return n.Kind.String() + " def"
	}
	return strings.TrimPrefix(fmt.Sprintf("%T", node), "*ast.")
}

// vertexName is the name a vertex answers to, "" for an anonymous one.
func vertexName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.PseudostateNode:
		return n.Name
	case *ast.StateRegion:
		return n.Name
	}
	return runtime.ActionNodeName(node)
}

// qualifiedName renders a qualified name as written, "" for nil.
func qualifiedName(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	return semantics.QualifiedNameText(qn)
}
