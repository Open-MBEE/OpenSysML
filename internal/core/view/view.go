// Package view renders a SysML view: it turns the elements a view exposes into
// a rendering artifact, in the form the view's `render` member states.
//
// SysML v2 §10.2 states which rendering a view uses and leaves how a tool
// carries it out to the tool, so everything this package produces — the text
// form and the Mermaid form alike — is tool-defined output rather than a
// notation the specification defines. What is read from the model is not:
// the exposed set comes from semantics.Model.ExposedElements, connections from
// the model's own connector information, and states and actions from the
// lowered graphs in internal/core/lower, never from the source text of a
// declaration.
package view

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Kind is a rendering a view can state. The kinds this package produces are
// tree, interconnection, state, action, table and sequence; the rest are
// recognized so that a view stating one is told it is unsupported rather than
// rendered as something else. A rendering the standard library does not declare is carried
// as the name the model gives it, so an error about it names what the view
// asked for.
type Kind string

const (
	// KindTree renders the exposed elements as a containment tree.
	KindTree Kind = "tree"
	// KindInterconnection renders exposed features as nodes and the connections
	// between them as edges.
	KindInterconnection Kind = "interconnection"
	// KindState renders the states and transitions of exposed behaviors.
	KindState Kind = "state"
	// KindAction renders the nodes and successions of exposed behaviors.
	KindAction Kind = "action"
	// KindTextual is Views::asTextualNotation, which writes the model back as
	// notation: `sysml -convert sysml` does that, so no rendering is produced.
	KindTextual Kind = "textual"
	// KindTable renders the exposed elements as rows of a table, which is
	// Views::asElementTable and StandardViewDefinitions::GridView.
	KindTable Kind = "table"
	// KindSequence renders exposed occurrences as lifelines and the flows
	// between them as ordered messages, which is
	// StandardViewDefinitions::SequenceView.
	KindSequence Kind = "sequence"
	// KindGeometry is StandardViewDefinitions::GeometryView.
	KindGeometry Kind = "geometry"
)

// Kinds returns every rendering kind this package recognizes, supported or not.
func Kinds() []Kind {
	return []Kind{KindTree, KindInterconnection, KindState, KindAction, KindTextual, KindTable, KindSequence, KindGeometry}
}

// Supported reports whether this package produces a rendering of the kind.
func (k Kind) Supported() bool {
	switch k {
	case KindTree, KindInterconnection, KindState, KindAction, KindTable, KindSequence:
		return true
	}
	return false
}

// article is the indefinite article the kind reads with, so a message says "an
// action rendering" rather than "a action rendering".
func (k Kind) article() string {
	switch {
	case k == "":
		return "a"
	case strings.ContainsRune("aeiou", rune(k[0])):
		return "an"
	}
	return "a"
}

// ErrUnsupportedKind is the rendering kind a view states that this package does
// not produce. UnsupportedKindError wraps it, so a caller can test for it
// without knowing which kind was asked for.
var ErrUnsupportedKind = errors.New("unsupported rendering kind")

// UnsupportedKindError is a view stating a rendering kind that is recognized but
// not produced. It names both the kind and the view, and never falls back to
// another kind.
type UnsupportedKindError struct {
	// Kind is the rendering kind asked for.
	Kind Kind
	// View is the view stating it, by qualified name.
	View string
	// Stated is what the model says the kind through — a rendering member, or
	// the standard view definition the view specializes.
	Stated string
	// Remedy is what to do instead, empty when there is nothing to suggest.
	Remedy string
}

func (e *UnsupportedKindError) Error() string {
	msg := fmt.Sprintf("%s: %s rendering is not supported", e.View, e.Kind)
	if e.Stated != "" {
		msg = fmt.Sprintf("%s: %s rendering (%s) is not supported", e.View, e.Kind, e.Stated)
	}
	if e.Remedy != "" {
		return msg + "; " + e.Remedy
	}
	return msg
}

func (e *UnsupportedKindError) Unwrap() error { return ErrUnsupportedKind }

// SourceText answers the notation a span of a document was written in, so that a
// label a rendering takes verbatim — a transition guard, a trigger expression —
// reads as it was written. It may be nil, and returns "" for a document it does
// not hold, in which case such a label is described structurally instead.
type SourceText = source.Lookup

// Renderer renders views over one semantic model.
type Renderer struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	text     SourceText
}

// NewRenderer returns a renderer over the model and resolver of a loaded
// document. text may be nil.
func NewRenderer(model *semantics.Model, resolver *resolve.Resolver, text SourceText) *Renderer {
	return &Renderer{model: model, resolver: resolver, text: text}
}

// EdgeKind classifies what an edge of a rendering stands for.
type EdgeKind int

const (
	// EdgeConnection is a connector joining two features.
	EdgeConnection EdgeKind = iota
	// EdgeTransition is a state transition.
	EdgeTransition
	// EdgeSuccession is a succession between action nodes.
	EdgeSuccession
	// EdgeFlow is a flow of a payload between action nodes.
	EdgeFlow
)

// String names an edge kind the way the notation speaks of it.
func (k EdgeKind) String() string {
	switch k {
	case EdgeConnection:
		return "connection"
	case EdgeTransition:
		return "transition"
	case EdgeSuccession:
		return "succession"
	case EdgeFlow:
		return "flow"
	}
	return "edge"
}

// Node is one element of a rendering: an exposed element, a feature nested in
// one, a state or an action node, or the "start" of a state body, which its entry
// transitions leave. Children are the nodes nested in it, which is how a
// rendering carries containment.
type Node struct {
	// ID identifies the node within its rendering, and is what an edge names.
	ID string
	// Kind is what the notation calls the element — "part def", "state",
	// "fork" — never a Go type name.
	Kind string
	// Name is the element's name: qualified for a node the view exposes, simple
	// for one nested in it. It is empty for an anonymous element.
	Name string
	// Detail is what else the kind carries, such as a state's "initial" or the
	// type of a usage. It is empty when there is nothing to add.
	Detail string
	// Children are the nodes nested in this one.
	Children []*Node
	// Origin is where the element was declared, the zero Origin for one with no
	// locatable declaration.
	Origin Origin
	// Geometry is where the element is drawn, from the Layout annotation that
	// positions it in this view; nil leaves the placement to the writer.
	Geometry *Geometry
}

// Edge joins two nodes of a rendering.
type Edge struct {
	// From and To are node IDs.
	From string
	To   string
	// Label is what the edge carries: a connector's name, a transition's
	// trigger, guard and effect, a succession's guard. It may be empty.
	Label string
	Kind  EdgeKind
	// Origin is where the connection, transition, succession or flow was
	// declared, the zero Origin for one with no locatable declaration.
	Origin Origin
	// Route is the waypoints the edge follows, from the Route annotation of the
	// element it was declared as; empty leaves the routing to the writer.
	Route []Point
}

// Rendering is what a view renders to: the nodes and edges of one artifact,
// which Text and Mermaid write out. It is a value, not a live view of the
// model: nothing in it points back into the AST.
type Rendering struct {
	// View is the rendered view, by qualified name as the notation writes it.
	View string
	// Kind is the rendering produced.
	Kind Kind
	// Stated is how the kind was decided: the rendering member the view states,
	// the standard view definition it specializes, or "" when the view states
	// nothing and the default was used.
	Stated string
	// Roots are the top-level nodes, in the order the view exposes them.
	Roots []*Node
	// Edges join nodes, in the order the model and the lowered graphs give them.
	Edges []Edge
	// Columns are the headings of a tabular rendering, empty for every other
	// kind.
	Columns []string
	// Rows are the rows of a tabular rendering, each holding one cell per
	// column, in the order the view exposes the elements.
	Rows [][]string
	// RowOrigins is where each row's element was declared, one entry per row.
	RowOrigins []Origin
	// Canvas is the drawing surface the view states, nil for a view stating
	// none.
	Canvas *Canvas
	// Notices are what the rendering could not represent, reported rather than
	// dropped: an exposed element with no place in this kind of rendering, a
	// connection to something the view does not expose, a behavior that does not
	// lower.
	Notices []string

	// drawn collects the elements drawn while rendering, nil when no one asked.
	drawn *Drawn
}

// Empty reports whether the rendering has nothing to show.
func (r *Rendering) Empty() bool {
	return len(r.Roots) == 0 && len(r.Edges) == 0 && len(r.Rows) == 0
}

// Render renders view in the kind it states, defaulting to a tree when it
// states none. A view that is no view is semantics.ErrNotAView, and a
// recognized kind this package does not produce is an *UnsupportedKindError —
// never another kind's rendering.
func (r *Renderer) Render(view *symbols.Symbol) (*Rendering, error) {
	return r.render(view, nil)
}

// render is Render, collecting what is drawn into drawn when it is not nil.
func (r *Renderer) render(view *symbols.Symbol, drawn *Drawn) (*Rendering, error) {
	kind, stated, err := r.KindOf(view)
	if err != nil {
		return nil, err
	}
	exposed, err := r.model.ExposedElements(view)
	if err != nil {
		return nil, err
	}
	out := &Rendering{View: r.notationName(view), Kind: kind, Stated: stated, drawn: drawn}
	switch kind {
	case KindTree:
		r.renderTree(view, exposed, out)
	case KindInterconnection:
		r.renderInterconnection(view, exposed, out)
	case KindState:
		r.renderStates(view, exposed, out)
	case KindAction:
		r.renderActions(view, exposed, out)
	case KindTable:
		r.renderTable(view, exposed, out)
	case KindSequence:
		r.renderSequence(exposed, out)
	default:
		// Unreachable: KindOf refuses an unsupported kind.
		return nil, &UnsupportedKindError{Kind: kind, View: r.notationName(view), Stated: stated}
	}
	switch kind {
	case KindTree, KindInterconnection, KindState, KindAction:
		// The graph-shaped kinds are drawn on a canvas; a table or sequence is not.
		out.Canvas = r.canvasOf(view, out)
	}
	return out, nil
}

// RenderExposed renders elements in the kind asked for, as a view exposing just
// them would: a document with no view declared is rendered this way. Nothing is
// added to the model or to the symbol index, so the rendering carries no view
// name; stated says how the kind was decided. An unsupported kind is an
// *UnsupportedKindError.
func (r *Renderer) RenderExposed(exposed []*symbols.Symbol, kind Kind, stated string) (*Rendering, error) {
	out := &Rendering{Kind: kind, Stated: stated}
	switch kind {
	case KindTree:
		r.renderTree(nil, exposed, out)
	case KindInterconnection:
		r.renderInterconnection(nil, exposed, out)
	case KindState:
		r.renderStates(nil, exposed, out)
	case KindAction:
		r.renderActions(nil, exposed, out)
	case KindTable:
		r.renderTable(nil, exposed, out)
	case KindSequence:
		r.renderSequence(exposed, out)
	default:
		return nil, &UnsupportedKindError{Kind: kind, Stated: stated, Remedy: remedyFor(kind)}
	}
	return out, nil
}

// KindOf reports the rendering kind a view states and how it states it: the
// rendering its body names, else the standard view definition it specializes,
// else the tree every view defaults to. A recognized kind this package does not
// produce is an *UnsupportedKindError.
func (r *Renderer) KindOf(view *symbols.Symbol) (Kind, string, error) {
	if view == nil || !semantics.IsView(view) {
		return "", "", semantics.ErrNotAView
	}
	renderings, err := r.model.ViewRenderings(view)
	if err != nil {
		return "", "", err
	}
	definitionKind, definitionStated := r.viewDefinitionKind(view)
	for _, rendering := range renderings {
		kind, ok := r.renderingKind(rendering)
		if !ok {
			continue
		}
		stated := fmt.Sprintf("render %s", notationName(rendering.Ref))
		if rendering.Ref == "" {
			stated = "the rendering the view declares"
		}
		// An interconnection diagram of a view definition that presents states or
		// actions is that graph: StandardViewDefinitions says so, and
		// StateTransitionView and ActionFlowView both specialize
		// InterconnectionView.
		if kind == KindInterconnection && (definitionKind == KindState || definitionKind == KindAction) {
			return definitionKind, fmt.Sprintf("%s, %s", stated, definitionStated), nil
		}
		if !kind.Supported() {
			return "", "", &UnsupportedKindError{Kind: kind, View: r.notationName(view), Stated: stated, Remedy: remedyFor(kind)}
		}
		return kind, stated, nil
	}
	if definitionKind != "" {
		if !definitionKind.Supported() {
			return "", "", &UnsupportedKindError{
				Kind: definitionKind, View: r.notationName(view), Stated: definitionStated, Remedy: remedyFor(definitionKind),
			}
		}
		return definitionKind, definitionStated, nil
	}
	return KindTree, "", nil
}

// remedyFor is what to do instead of a rendering kind this package does not
// produce, empty for one nothing else answers.
func remedyFor(kind Kind) string {
	switch kind {
	case KindTextual:
		return "the notation itself is written by `sysml <model> -convert sysml`"
	}
	return ""
}

// The standard library packages the recognized renderings and view definitions
// are declared in. A rendering or view definition of the same name declared
// elsewhere is the model's own, not one of these.
const (
	renderingsPackage      = "Views::"
	viewDefinitionsPackage = "StandardViewDefinitions::"
)

// standardRenderings maps the renderings Views declares, and the rendering
// definitions they are typed by, to the kind each asks for. An abstract base
// rendering asks for no kind, which is the empty kind here.
var standardRenderings = map[string]Kind{
	"asTreeDiagram":            KindTree,
	"asInterconnectionDiagram": KindInterconnection,
	"asTextualNotation":        KindTextual,
	"asElementTable":           KindTable,
	"TextualRendering":         KindTextual,
	"TabularRendering":         KindTable,
	"GraphicalRendering":       "",
	"Rendering":                "",
}

// standardViewDefinitions maps the standard view definitions, by name and by
// short name, to the kind each presents. A view specializing one is rendered
// that way unless it states a rendering of its own.
var standardViewDefinitions = map[string]Kind{
	"StateTransitionView": KindState, "stv": KindState,
	"ActionFlowView": KindAction, "afv": KindAction,
	"InterconnectionView": KindInterconnection, "iv": KindInterconnection,
	"GeneralView": KindTree, "gv": KindTree,
	"BrowserView": KindTree, "bv": KindTree,
	"SequenceView": KindSequence, "sv": KindSequence,
	"GeometryView": KindGeometry, "gev": KindGeometry,
	"GridView": KindTable, "grv": KindTable,
}

// standardKind looks a qualified name up in one of the tables above: the
// element must be the one the standard library package declares under that
// name, so a same-named declaration of the model's own is not mistaken for it.
func standardKind(table map[string]Kind, pkg, fqn string) (Kind, bool) {
	if !strings.HasPrefix(fqn, pkg) {
		return "", false
	}
	kind, ok := table[strings.TrimPrefix(fqn, pkg)]
	return kind, ok
}

// renderingKind reports the kind a rendering member asks for, and whether it
// asks for one at all: an abstract base rendering (`Rendering`,
// `GraphicalRendering`) states no kind, and neither does a member naming
// nothing. A rendering the standard library does not declare is read through
// what it specializes, and is unsupported when that says nothing either.
func (r *Renderer) renderingKind(rendering semantics.ViewRendering) (Kind, bool) {
	if rendering.Rendering == nil {
		if rendering.Ref == "" {
			return "", false
		}
		// A rendering that does not resolve is a name error the analysis reports;
		// rendering it as something else would hide that, so it is unsupported.
		return Kind(rendering.Ref), true
	}
	for _, sym := range append([]*symbols.Symbol{rendering.Rendering}, r.model.AllSupertypes(rendering.Rendering)...) {
		kind, known := standardKind(standardRenderings, renderingsPackage, r.fqn(sym))
		if !known {
			continue
		}
		if kind == "" {
			return "", false
		}
		return kind, true
	}
	return Kind(simpleName(r.fqn(rendering.Rendering))), true
}

// viewDefinitionKind reports the kind the standard view definition a view
// specializes presents, and how it says so. The nearest supertype decides, so a
// StateTransitionView is a state rendering and not the interconnection view it
// in turn specializes.
func (r *Renderer) viewDefinitionKind(view *symbols.Symbol) (Kind, string) {
	for _, sym := range append([]*symbols.Symbol{view}, r.model.AllSupertypes(view)...) {
		fqn := r.fqn(sym)
		if kind, ok := standardKind(standardViewDefinitions, viewDefinitionsPackage, fqn); ok {
			return kind, "view def " + simpleName(fqn)
		}
	}
	return "", ""
}

// nodeIDs hands out the identities a rendering's nodes are named by, which the
// machine-readable form uses in place of a name that may need quoting.
type nodeIDs struct {
	next int
}

func (n *nodeIDs) take() string {
	id := fmt.Sprintf("n%d", n.next)
	n.next++
	return id
}

// fqn is the qualified name the index spells a symbol with, falling back to the
// name the symbol carries — which is already qualified for a cached library
// symbol.
func (r *Renderer) fqn(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if idx := r.resolver.Index(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}

// notationName is a symbol's qualified name as the notation writes it, with the
// quotes an unrestricted name needs.
func (r *Renderer) notationName(sym *symbols.Symbol) string {
	return notationName(r.fqn(sym))
}

// declKind names an element the way the notation declares it — "part def",
// "state", "view" — so a rendering prints no Go type name.
func declKind(sym *symbols.Symbol) string {
	return sym.Notation()
}

// declType is the type a usage is declared with, as written ("Engine" of
// `part engine : Engine`), empty for a declaration stating none.
func declType(sym *symbols.Symbol) string {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind == ast.RelTyping {
			return qualifiedText(rel.Target)
		}
	}
	return ""
}

// simpleName is the last segment of a qualified name, which is what a nested
// node is labeled with.
func simpleName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+2:]
	}
	return fqn
}

// notationName writes a qualified name as the notation does, with the one
// quoting rule the REPL prints names by, so `%render` and `%view` spell the same
// element identically.
func notationName(fqn string) string {
	return lexer.QualifiedNameText(fqn)
}

// qualifiedText renders a name reference as it was written, dotted chains
// included.
func qualifiedText(node ast.Node) string {
	if qn := ast.AsQualifiedName(node); qn != nil {
		parts := make([]string, 0, len(qn.Parts))
		for _, part := range qn.Parts {
			parts = append(parts, part.Text)
		}
		return strings.Join(parts, "::")
	}
	return ast.SimpleName(node)
}
