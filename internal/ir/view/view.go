// Package view renders a SysML view: it turns the elements a view exposes into
// a rendering artifact, in the form the view's `render` member states.
//
// SysML v2 §10.2 states which rendering a view uses and leaves how a tool
// carries it out to the tool, so everything this package produces — the text
// form and the Mermaid form alike — is tool-defined output rather than a
// notation the specification defines. What is read from the model is not:
// ordinary renderings use semantics.Model.ExposedElements, while a matrix opts
// into semantics.Model.ExposedMembers; connections come from the model's own
// connector information, and states and actions from the lowered graphs in
// internal/ir/lower, never from the source text of a declaration.
package view

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Kind is a rendering a view can state. The kinds this package produces are
// tree, interconnection, state, action, case, mixed, table, matrix, sequence
// and the requirement, definition and package graphs a filtered GeneralView
// presents; the rest are recognized so that a view stating one is told it is
// unsupported rather than rendered as something else. A rendering the standard
// library does not declare is carried as the name the model gives it, so an
// error about it names what the view asked for.
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
	// KindCase names SysML's CaseDefinition/CaseUsage family; "usecase" would
	// mislabel analysis and verification cases.
	KindCase Kind = "case"
	// KindMixed combines behavioral, case, structural, and tree content on one canvas.
	KindMixed Kind = "mixed"
	// KindTextual is Views::asTextualNotation, which writes the model back as
	// notation: `sysml -convert sysml` does that, so no rendering is produced.
	KindTextual Kind = "textual"
	// KindTable renders the exposed elements as rows of a table, which is
	// Views::asElementTable and an unfiltered StandardViewDefinitions::GridView.
	KindTable Kind = "table"
	// KindMatrix renders the relationships selected by a filtered GridView.
	KindMatrix Kind = "matrix"
	// KindSequence renders exposed occurrences as lifelines and the flows
	// between them as ordered messages, which is
	// StandardViewDefinitions::SequenceView.
	KindSequence Kind = "sequence"
	// KindTimeline renders a run's trace as each object machine's states over
	// clock time; no view states it.
	KindTimeline Kind = "timeline"
	// KindGeometry is StandardViewDefinitions::GeometryView.
	KindGeometry Kind = "geometry"
	// KindRequirement renders exposed requirements as nodes and the
	// relationships between them and what satisfies, verifies, derives, refines
	// or is allocated to them as edges: a GeneralView filtered to requirements.
	KindRequirement Kind = "requirement"
	// KindDefinition renders exposed definitions and usages as nodes and their
	// specialization, typing, composition and reference as edges: a GeneralView
	// filtered to definitions and usages.
	KindDefinition Kind = "definition"
	// KindPackage renders exposed packages as nodes and their containment and
	// imports as edges: a GeneralView filtered to packages.
	KindPackage Kind = "package"
)

// Kinds returns every rendering kind this package recognizes, supported or not.
func Kinds() []Kind {
	return []Kind{KindTree, KindInterconnection, KindState, KindAction, KindCase, KindMixed, KindTextual, KindTable, KindMatrix, KindSequence, KindGeometry,
		KindRequirement, KindDefinition, KindPackage}
}

// Supported reports whether this package produces a rendering of the kind.
func (k Kind) Supported() bool {
	switch k {
	case KindTree, KindInterconnection, KindState, KindAction, KindCase, KindMixed, KindTable, KindMatrix, KindSequence,
		KindRequirement, KindDefinition, KindPackage:
		return true
	}
	return false
}

// Tabular reports whether k is rendered as rows and columns.
func (k Kind) Tabular() bool { return k == KindTable || k == KindMatrix }

// GraphShaped reports whether k is drawn as a graph of nodes and edges: every
// produced kind that is not tabular, the kinds a document's diagrams and the
// DOT, Mermaid, PlantUML and D2 forms draw.
func (k Kind) GraphShaped() bool { return k.Supported() && !k.Tabular() }

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
	// treeDepthBound overrides the containment depth bound when set; tests only.
	treeDepthBound int
	// verdicts overlays verification verdicts on a requirement rendering; nil
	// draws it structurally.
	verdicts Verdicts
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
	// EdgeBinding is a binding equating two features.
	EdgeBinding
	// EdgeSpecialization is a specialization, subsetting or redefinition, from
	// the specific element to the general one.
	EdgeSpecialization
	// EdgeTyping is a feature typing, from a usage to its definition.
	EdgeTyping
	// EdgeComposition is a composite usage, from its owner to it; in a case
	// diagram, a case contained in another.
	EdgeComposition
	// EdgeReference is a referential usage, from its owner to it; in a case or
	// mixed diagram, a performing or exhibiting usage to its target.
	EdgeReference
	// EdgeContainment is a package owning another, from the owner.
	EdgeContainment
	// EdgeImport is a package's import, from the importing package.
	EdgeImport
	// EdgeSatisfy is a satisfy relationship, from the satisfying element.
	EdgeSatisfy
	// EdgeVerify is a verification case verifying a requirement.
	EdgeVerify
	// EdgeDerive is a requirement derivation, from the derived requirement.
	EdgeDerive
	// EdgeRefine is a refinement dependency, from the refining element.
	EdgeRefine
	// EdgeAllocate is an allocation, from the allocated element.
	EdgeAllocate
	// EdgeAssociation connects an actor or subject to a case.
	EdgeAssociation
	// EdgeInclude includes one case in another.
	EdgeInclude
	// EdgeAnchor attaches an objective to a case.
	EdgeAnchor
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
	case EdgeBinding:
		return "binding"
	case EdgeSpecialization:
		return "specialization"
	case EdgeTyping:
		return "typing"
	case EdgeComposition:
		return "composition"
	case EdgeReference:
		return "reference"
	case EdgeContainment:
		return "containment"
	case EdgeImport:
		return "import"
	case EdgeSatisfy:
		return "satisfy"
	case EdgeVerify:
		return "verify"
	case EdgeDerive:
		return "derive"
	case EdgeRefine:
		return "refine"
	case EdgeAllocate:
		return "allocate"
	case EdgeAssociation:
		return "association"
	case EdgeInclude:
		return "include"
	case EdgeAnchor:
		return "anchor"
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
	// NameSynthesized marks a name the model did not give: a migration made it up, or
	// it is the language's `start`/`done`. A name to key by, not one a picture shows.
	NameSynthesized bool
	// StandIn marks a node a migration made up that stands for no element of its
	// source, so no diagram symbol is at it: a join it wrote several edges through.
	StandIn bool
	// Type is the declared type of a typed usage, as the notation writes it
	// after the colon. It is empty for a definition or an untyped usage.
	Type string
	// Typings are the qualified names of the elements Type resolves to, in its
	// order; a member of one is drawn as the type's own. Empty when none resolves.
	Typings []string
	// Detail is what else the rendering says about the node, such as a state's
	// "initial" or "already shown". It is empty when there is nothing to add.
	Detail string
	// Text is what heads a node whose name is not shown, in place of its kind:
	// the literal a value specification's result is bound to, the event an
	// accept waits for, the message a send sends. Empty when its kind heads it.
	Text string
	// Children are the nodes nested in this one.
	Children []*Node
	// Ports are the features drawn on the node's border, an action's pins, which
	// an edge may end at instead of the node itself.
	Ports []Port
	// Origin is where the element was declared, the zero Origin for one with no
	// locatable declaration.
	Origin Origin
	// Inherited is where the declarations a drawn behavior took content from besides
	// its own were written (what it specializes or is typed by); only a root has them.
	Inherited []Origin
	// Geometry is where the element is drawn, from the Layout annotation that
	// positions it in this view; nil leaves the placement to the writer.
	Geometry *Geometry
	// Style is how the element is drawn, from the Style annotation colouring it
	// in this view, or from its verdict; nil leaves the look to the drawing style.
	Style *Style
	// Verdict is the worst verdict of the verification cases verifying a
	// requirement, when the rendering overlays them: pass, inconclusive, fail or
	// error. Empty when none is overlaid.
	Verdict string
	// verdictStyled marks a Style taken from the Verdict, not from the model.
	verdictStyled bool
}

// Port is a feature drawn on a node's border: an input or output pin of an
// action, which an object flow ends at, or a port a part has from its
// definition, which a connector ends at.
type Port struct {
	// ID identifies the port within its rendering, and is what an edge names.
	ID string
	// Name is the pin's name, as the notation writes it.
	Name string
	// NameSynthesized marks a name the model did not give, which a form
	// leaves off the pin's label as it does off a node's.
	NameSynthesized bool
	// Type is the port's declared type as the notation writes it, `~T` for a
	// conjugated one; empty for a pin, whose type the flow's label carries.
	Type string
	// Direction is the pin's direction: `in`, `out` or `inout`; PortUndirected
	// for a part's port, whose features carry the directions.
	Direction PortDirection
	// Origin is where the pin was declared, the zero Origin for one with no
	// locatable declaration.
	Origin Origin
}

// PortDirection is which way a port's values flow.
type PortDirection int

const (
	// PortIn takes values in.
	PortIn PortDirection = iota
	// PortOut gives values out.
	PortOut
	// PortInOut does both.
	PortInOut
	// PortUndirected has no direction of its own: a part's port.
	PortUndirected
)

// String writes a port direction as the notation does, "" for none.
func (d PortDirection) String() string {
	switch d {
	case PortOut:
		return "out"
	case PortInOut:
		return "inout"
	case PortUndirected:
		return ""
	}
	return "in"
}

// label is the text a port is drawn with: its name, and its type where it has one.
func (p Port) label() string {
	name := p.Name
	if p.NameSynthesized {
		name = ""
	}
	if p.Type == "" {
		return name
	}
	return name + " : " + p.Type
}

// Edge joins two nodes of a rendering.
type Edge struct {
	// From and To are node IDs.
	From string
	To   string
	// FromPort and ToPort are the IDs of the ports of From and To the edge ends
	// at, empty where it ends at the node itself.
	FromPort string
	ToPort   string
	// Label is what the edge carries: a connector's name, a transition's
	// trigger, guard and effect, a succession's guard, a flow's pins. It may be empty.
	Label string
	// Name is the edge's own name, for a writer whose drawing shows the rest
	// of its Label another way; empty for one anonymous or named by a migration.
	Name string
	Kind EdgeKind
	// Origin is where the connection, transition, succession or flow was
	// declared, the zero Origin for one with no locatable declaration.
	Origin Origin
	// Route is the waypoints the edge follows, from the Route annotation of the
	// element it was declared as; empty leaves the routing to the writer.
	Route []Point
	// Style is how the edge is drawn, from the Style annotation colouring it;
	// nil leaves the look to the drawing style.
	Style *Style
	// openFrom and openTo mark a route that stops short of that end, at the
	// place of a node elided from between them; the drawing leads it on.
	openFrom, openTo bool
}

// Rendering is what a view or run renders to. It is a value, not a live view of
// the model: nothing in it points back into the AST.
type Rendering struct {
	// View is the rendered view, by qualified name as the notation writes it.
	View string
	// Kind is the rendering produced.
	Kind Kind
	// Stated is how the kind was decided: the rendering member the view states,
	// the standard view definition it specializes, or "" when the view states
	// nothing and the default was used.
	Stated string
	// Run marks a rendering of a run's trace rather than of a view.
	Run bool
	// RunUntil is the clock instant through which a run rendering was recorded.
	RunUntil float64
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
	// Lanes are a timeline's object machines, in the order the run first
	// recorded them.
	Lanes []Lane
	// RowOrigins is where each row's element was declared, one entry per row.
	RowOrigins []Origin
	// Canvas is the drawing surface the view states, nil for a view stating
	// none.
	Canvas *Canvas
	// Notes are the note boxes drawn on the canvas, anchored to a node or free,
	// in the order the nodes they annotate are drawn, free ones last.
	Notes []Note
	// Pictures are the pictures drawn on the canvas, in the order the view
	// states them; a later one is drawn over an earlier one it overlaps.
	Pictures []Picture
	// Notices are what the rendering could not represent, reported rather than
	// dropped: an exposed element with no place in this kind of rendering, a
	// connection to something the view does not expose, a behavior that does not
	// lower.
	Notices []string

	emptyReason string
	// drawn collects the elements drawn while rendering, nil when no one asked.
	drawn *Drawn
	// sites records, while a tree renders, what each of its nodes draws; nil
	// once its edges are drawn and for every other kind.
	sites map[*Node]treeSite
}

// Lane is one object machine in a run timeline.
type Lane struct {
	ID   string
	Name string
	// Origin is where the lane's state machine was declared.
	Origin      Origin
	Spans       []Span
	Transitions []LaneTransition
	Marks       []Mark
}

// Span is a state's occupancy of a lane over clock time.
type Span struct {
	State string
	// Origin is where the one state shown by the span was declared.
	Origin   Origin
	From, To float64
	Triggers []string
	Through  []string
	Open     bool
}

// LaneTransition is a transition recorded at an instant in a lane.
type LaneTransition struct {
	At       float64
	From, To string
	Event    string
}

// Mark is a choice or guard record attached to a lane at an instant.
type Mark struct {
	At   float64
	Kind string
	Text string
}

// Empty reports whether the rendering has nothing to show.
func (r *Rendering) Empty() bool {
	return len(r.Roots) == 0 && len(r.Edges) == 0 && len(r.Rows) == 0 && len(r.Pictures) == 0 && len(r.Lanes) == 0
}

// Positioned reports whether a Layout or Route places some node of a
// graph-shaped rendering, or a Picture is drawn on it at stated bounds, so
// the DOT form draws it where the diagram states.
func (r *Rendering) Positioned() bool {
	return r != nil && r.Kind.SupportsForm(FormDot) && (placeRendering(r).positioned())
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
	var exposed []*symbols.Symbol
	if kind == KindMatrix {
		exposed, err = r.model.ExposedMembers(view)
	} else {
		exposed, err = r.model.ExposedElements(view)
	}
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
	case KindCase:
		r.renderCase(view, exposed, out)
	case KindMixed:
		r.renderMixed(view, exposed, out)
	case KindTable:
		r.renderTable(view, exposed, out)
	case KindMatrix:
		r.renderMatrix(exposed, r.matrixShownKinds(view), false, out)
	case KindSequence:
		r.renderSequence(exposed, out)
	case KindRequirement, KindDefinition, KindPackage:
		r.renderGeneral(view, exposed, out)
	default:
		// Unreachable: KindOf refuses an unsupported kind.
		return nil, &UnsupportedKindError{Kind: kind, View: r.notationName(view), Stated: stated}
	}
	switch kind {
	case KindTree, KindInterconnection, KindState, KindAction, KindCase, KindMixed, KindRequirement, KindDefinition, KindPackage:
		// The graph-shaped kinds are drawn on a canvas; a table or sequence is not.
		out.Canvas = r.canvasOf(view, out)
		r.notesOf(view, view, "", out)
		r.picturesOf(view, out)
	case KindTable, KindMatrix, KindSequence:
		r.undrawnPicturesOf(view, out)
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
	case KindCase:
		r.renderCase(nil, exposed, out)
	case KindMixed:
		r.renderMixed(nil, exposed, out)
	case KindTable:
		r.renderTable(nil, exposed, out)
	case KindMatrix:
		r.renderMatrix(exposed, allMatrixKinds(), true, out)
	case KindSequence:
		r.renderSequence(exposed, out)
	case KindRequirement, KindDefinition, KindPackage:
		r.renderGeneral(nil, exposed, out)
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
	definitionKind, definitionStated, definition := r.viewDefinitionKind(view)
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
		if definition != nil && r.fqn(definition) == viewDefinitionsPackage+"GridView" {
			if shown := r.matrixShownKinds(view); len(shown) != 0 {
				return KindMatrix, definitionStated, nil
			}
		}
		if general, filter, ok := r.generalSpecialization(view); ok {
			return general, fmt.Sprintf("%s, filter %s", definitionStated, filter), nil
		}
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
	toolRenderingsPackage  = "OpenSysMLRenderings::"
)

var toolRenderings = map[string]Kind{
	"asCaseDiagram":  KindCase,
	"asMixedDiagram": KindMixed,
}

var toolViewDefinitions = map[string]Kind{
	"CaseView":  KindCase,
	"MixedView": KindMixed,
}

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
		if kind, known := standardKind(toolRenderings, toolRenderingsPackage, r.fqn(sym)); known {
			return kind, true
		}
		kind, known := standardKind(standardRenderings, renderingsPackage, r.fqn(sym))
		if !known {
			continue
		}
		if kind == "" {
			return "", false
		}
		return kind, true
	}
	return Kind(rendering.Rendering.Name), true
}

// viewDefinitionKind reports the kind the standard view definition a view
// specializes presents, and how it says so. The nearest supertype decides, so a
// StateTransitionView is a state rendering and not the interconnection view it
// in turn specializes.
func (r *Renderer) viewDefinitionKind(view *symbols.Symbol) (Kind, string, *symbols.Symbol) {
	for _, sym := range append([]*symbols.Symbol{view}, r.model.AllSupertypes(view)...) {
		if kind, ok := standardKind(toolViewDefinitions, toolRenderingsPackage, r.fqn(sym)); ok {
			return kind, "view def " + sym.Name, sym
		}
		if kind, ok := standardKind(standardViewDefinitions, viewDefinitionsPackage, r.fqn(sym)); ok {
			return kind, "view def " + sym.Name, sym
		}
	}
	return "", "", nil
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

// notationName is a symbol's qualified name as the notation writes it, each
// segment quoted on its own from the owner chain, since a name may hold `::`.
func (r *Renderer) notationName(sym *symbols.Symbol) string {
	names := symbols.NameChain(sym)
	if len(names) == 0 || len(names) == 1 && names[0] == "" {
		return ""
	}
	return source.QualifiedNameOf(names)
}

// localName is a symbol's own name as the notation writes it, empty for an
// anonymous one.
func localName(sym *symbols.Symbol) string {
	return nameText(sym.Name)
}

// nameText is one name as the notation writes it, empty for no name.
func nameText(name string) string {
	if name == "" {
		return ""
	}
	return source.NameText(name)
}

// declKind names an element the way the notation declares it — "part def",
// "state", "view" — so a rendering prints no Go type name.
func declKind(sym *symbols.Symbol) string {
	return sym.Notation()
}

// declType is the type a usage is declared with, as written ("Engine" of
// `part engine : Engine`, "~Port" of `port p : ~Port`, "A, B" of
// `feature f typed by A, B`), empty for a declaration stating none.
func declType(sym *symbols.Symbol) string {
	return typingOf(viewRelationshipsOf(sym))
}

// declTypings are the qualified names, as the notation writes them, of the
// elements a usage's declared typings resolve to.
func (r *Renderer) declTypings(sym *symbols.Symbol) []string {
	var names []string
	for _, typ := range r.model.DeclaredTypes(sym) {
		if name := r.notationName(typ); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// nodeType is the type a usage lowered into a behavior graph is declared with
// ("Provide" of `action provide : Provide`), empty for any other node.
func nodeType(decl ast.Node) string {
	if usage, ok := decl.(*ast.Usage); ok {
		return typingOf(usage.Relationships)
	}
	return ""
}

// typingOf spells a declaration's typings as the notation does: every typing in
// declaration order, a conjugated one behind its `~`, each name quoted as needed.
func typingOf(rels []*ast.Relationship) string {
	var types []string
	for _, rel := range rels {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelTyping {
			continue
		}
		text := referenceText(rel.Target)
		if text == "" {
			continue
		}
		if rel.Conjugated {
			text = "~" + text
		}
		types = append(types, text)
	}
	return strings.Join(types, ", ")
}

// referenceText spells a name reference as written: a `$::` root, `::` between
// members, `.` along a feature chain, and each segment quoted as needed.
func referenceText(node ast.Node) string {
	if chain, ok := node.(*ast.FeatureChainExpr); ok {
		return referenceText(chain.Operand) + "." + referenceText(chain.Member)
	}
	qn := ast.AsQualifiedName(node)
	if qn == nil {
		return ""
	}
	var sb strings.Builder
	if qn.Global {
		sb.WriteString("$::")
	}
	for i, part := range qn.Parts {
		switch {
		case i == 0:
		case part.Chained:
			sb.WriteString(".")
		default:
			sb.WriteString("::")
		}
		sb.WriteString(source.NameText(part.Text))
	}
	return sb.String()
}

// simpleName is the last segment of a qualified name, which is what a nested
// node is labeled with.
func simpleName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+2:]
	}
	return fqn
}

// notationName writes a reference read as joined qualified text (a `render`
// target, an accepted signal, a `via` port) as the notation does.
func notationName(fqn string) string {
	return source.QualifiedNameText(fqn)
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
