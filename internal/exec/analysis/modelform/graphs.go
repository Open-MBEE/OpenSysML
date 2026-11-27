package modelform

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
var ErrGraphsSubject = errors.New("no lowered graph")

// ErrGraphsOrder reports two graph elements the export cannot tell apart, which
// would make the output depend on memory layout; the lowering never produces them.
var ErrGraphsOrder = errors.New("graphs: elements of one graph cannot be ordered")

// GraphsOf exports the lowered graph of subject, an action or a state machine
// declared in model, and of every behavior it performs, at GraphsVersion.
func GraphsOf(model *runtime.Model, subject *symbols.Symbol) (*Graphs, error) {
	if model == nil || subject == nil {
		return nil, ErrGraphsSubject
	}
	if subject.Decl == nil {
		return nil, noGraphFor(subject)
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
	// exprs is the expression table of the graph being written.
	exprs *lower.ExprTable

	lowered  map[string]bool // qualified names already exported or queued
	pending  []*symbols.Symbol
	machines map[*symbols.Symbol]bool // queued symbols that are state machines
}

// lowerSubject exports the subject itself, refusing one that is neither an
// action nor a state machine or whose lowering fails.
func (x *graphsExporter) lowerSubject(out *Graphs, sym *symbols.Symbol) error {
	x.lowered[symbols.FQNOf(sym)] = true
	switch {
	case sym.Kind == symbols.SymbolActionDef || sym.Kind.IsActionUsage():
		graph, err := lower.ToActionGraphWith(sym.Decl, runtime.DeclScope(sym), x.model.Resolver())
		if err != nil {
			return fmt.Errorf("%w: %w", noGraphFor(sym), err)
		}
		form, err := x.actionForm(sym, graph)
		if err != nil {
			return err
		}
		out.Actions = append(out.Actions, form)
	case sym.Kind == symbols.SymbolStateDef || sym.Kind == symbols.SymbolStateUsage:
		graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, runtime.DeclScope(sym),
			lower.NewLibraryStateTypes(x.model.Resolver()))
		if err != nil {
			return fmt.Errorf("%w: %w", noGraphFor(sym), err)
		}
		form, err := x.stateForm(sym, graph)
		if err != nil {
			return err
		}
		out.States = append(out.States, form)
	default:
		return fmt.Errorf("%w: the subject must be an action or a state machine", noGraphFor(sym))
	}
	return nil
}

// noGraphFor is ErrGraphsSubject naming the subject once, by its kind in words
// and its name as the notation writes it, or by its owners when one is unnamed.
func noGraphFor(sym *symbols.Symbol) error {
	return fmt.Errorf("%w for the %s", ErrGraphsSubject, describeSubject(sym))
}

// describeSubject is sym's kind and qualified name, or, where an unnamed owner
// leaves it none, its own name and each owner out to the first that has one.
func describeSubject(sym *symbols.Symbol) string {
	if name := GraphsSubjectName(sym); name != "" {
		return sym.Kind.Phrase() + " " + name
	}
	own := "unnamed " + sym.Kind.Phrase()
	if sym.Named() {
		own = sym.Kind.Phrase() + " " + source.QualifiedNameOf([]string{sym.Name})
	}
	owner := ownerOf(sym)
	switch {
	case owner == nil:
		return own
	case GraphsSubjectName(owner) != "":
		return own + " of " + GraphsSubjectName(owner)
	case owner.Named():
		return own + " of the " + describeSubject(owner)
	}
	return own + " of an " + describeSubject(owner)
}

// GraphsSubjectName is the qualified name a refusal writes for sym, "" when sym or an
// owner is unnamed. A caller asked for sym by another spelling leads its report with it.
func GraphsSubjectName(sym *symbols.Symbol) string {
	for at := sym; at != nil; at = ownerOf(at) {
		if !at.Named() {
			return ""
		}
	}
	if sym == nil {
		return ""
	}
	return source.QualifiedNameOf(symbols.NameChain(sym))
}

// ownerOf is the symbol that declares sym, nil for a top-level one.
func ownerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
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
	graph, err := lower.ToActionGraphWith(sym.Decl, runtime.DeclScope(sym), x.model.Resolver())
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

// perform queues the behaviors a node performs or is typed by — a `perform`, a
// typed action usage, an exhibited state — once each, and returns their names.
func (x *graphsExporter) perform(performed []*symbols.Symbol) []string {
	var names []string
	for _, sym := range performed {
		if sym.Decl == nil {
			continue
		}
		machine := false
		switch sym.Kind {
		case symbols.SymbolStateDef, symbols.SymbolStateUsage:
			machine = true
		case symbols.SymbolActionDef:
		default:
			if !sym.Kind.IsActionUsage() {
				continue
			}
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

// expr renders the expression behind id in the table being written, read in
// scope; nil for none.
func (x *graphsExporter) expr(scope *symbols.Scope, id program.ExprID) *ExprForm {
	entry := x.exprs.Entry(id)
	if entry == nil {
		return nil
	}
	return x.exprAt(scope, entry.Extent())
}

// exprAt renders the expression written at an extent in scope; nil for none.
func (x *graphsExporter) exprAt(scope *symbols.Scope, at lower.Extent) *ExprForm {
	if !at.Written {
		return nil
	}
	span := at.Span
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
	if node := scope.Node(); !unwritten(node) {
		span := node.Span()
		return fmt.Sprintf("%s#%d+%d", scope.DocName(), span.Offset, span.Len)
	}
	return scope.DocName()
}

// unwritten reports a scope without a node, a typed nil pointer included.
func unwritten(node interface{ Span() source.Span }) bool {
	if node == nil {
		return true
	}
	v := reflect.ValueOf(node)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// vertexIDs numbers the vertices of one graph in an order that is a function of
// the lowered graph alone, so a table keyed by vertex is written as a list.
type vertexIDs[K comparable] struct {
	ids   map[K]int
	order []K
	// late reports a reference to a vertex the numbering never reached.
	late bool
}

func newVertexIDs[K comparable]() *vertexIDs[K] {
	return &vertexIDs[K]{ids: map[K]int{}}
}

// add numbers a vertex on first sight and returns its number; the zero K is -1.
func (v *vertexIDs[K]) add(vertex K) int {
	var none K
	if vertex == none {
		return -1
	}
	if id, ok := v.ids[vertex]; ok {
		return id
	}
	id := len(v.order)
	v.ids[vertex] = id
	v.order = append(v.order, vertex)
	return id
}

// ref is the number of a vertex, nil for none; a vertex first seen here is
// numbered too, so every reference resolves.
func (v *vertexIDs[K]) ref(vertex K) *int {
	var none K
	if vertex == none {
		return nil
	}
	id := v.add(vertex)
	return &id
}

// addSorted numbers the vertices not yet numbered by span, kind, name, then
// owner. Two the export cannot tell apart are refused, not written in table order.
func (v *vertexIDs[K]) addSorted(vertices []K, site func(K) (lower.Site, bool), owner func(K) string) error {
	var rest []K
	sites := make(map[K]lower.Site, len(vertices))
	keys := make(map[K]string, len(vertices))
	for _, vertex := range vertices {
		if _, ok := v.ids[vertex]; ok {
			continue
		}
		if _, ok := keys[vertex]; ok {
			continue
		}
		s, ok := site(vertex)
		if !ok {
			continue
		}
		sites[vertex] = s
		keys[vertex] = fmt.Sprintf("%012d:%012d:%s:%s:%s", s.Span.Offset, s.Span.Len, s.Kind, s.Name, owner(vertex))
		rest = append(rest, vertex)
	}
	sort.SliceStable(rest, func(i, j int) bool { return keys[rest[i]] < keys[rest[j]] })
	for i := 1; i < len(rest); i++ {
		if keys[rest[i]] == keys[rest[i-1]] {
			s := sites[rest[i]]
			return fmt.Errorf("%w: two %s vertices named %q at the same place", ErrGraphsOrder, s.Kind, s.Name)
		}
	}
	for _, vertex := range rest {
		v.add(vertex)
	}
	return nil
}
