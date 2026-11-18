package lower

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Site is what a declaration of a lowered graph is called and where it was written,
// as an export of the graph names it.
type Site struct {
	// Kind labels what was declared: `start`, `fork`, `action usage`, `pseudostate entry`.
	Kind string
	// Name is the name the declaration answers to, "" for an anonymous one.
	Name string
	Span source.Span
}

// absent reports no node at all, a typed nil pointer read from a table included.
func absent(node ast.Node) bool {
	if node == nil {
		return true
	}
	v := reflect.ValueOf(node)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// siteOf is the site of a declaration, false for none.
func siteOf(node ast.Node) (Site, bool) {
	if absent(node) {
		return Site{}, false
	}
	return Site{Kind: siteKind(node), Name: siteName(node), Span: node.Span()}, true
}

// Extent is where a declaration or an expression was written, the zero Extent for
// one lowering supplied or none at all.
type Extent struct {
	Span    source.Span
	Written bool
}

// extentOf is where node was written, the zero Extent for none.
func extentOf(node ast.Node) Extent {
	if absent(node) {
		return Extent{}
	}
	return Extent{Span: node.Span(), Written: true}
}

// siteKind labels a declaration by what it is in the lowered graph.
func siteKind(node ast.Node) string {
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

// siteName is the name a declaration answers to, "" for an anonymous one.
func siteName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.PseudostateNode:
		return n.Name
	case *ast.StateRegion:
		return n.Name
	}
	return originName(node)
}

// siteNames are the names of the declarations a path steps through.
func siteNames(path []ast.Node) []string {
	var names []string
	for _, node := range path {
		name := ""
		if !absent(node) {
			name = siteName(node)
		}
		names = append(names, name)
	}
	return names
}

// SiteOf is the site of the node behind id, false for no node.
func (g *ActionGraph) SiteOf(id program.NodeID) (Site, bool) {
	return siteOf(g.Provenance.Node(id))
}

// VertexSite is the site of the state, pseudostate or terminate action ref names.
func (g *StateGraph) VertexSite(ref program.VertexRef) (Site, bool) {
	return siteOf(g.vertexDecl(ref))
}

// RegionSite is the site of the region behind id, false for none.
func (g *StateGraph) RegionSite(id program.RegionID) (Site, bool) {
	return siteOf(g.Provenance.Region(id))
}

// At is where the succession was declared.
func (e ActionEdge) At() Extent { return extentOf(e.Decl) }

// GateAt is where the succession guarding the flow's transfer was declared.
func (e ActionEdge) GateAt() Extent { return extentOf(e.Gate) }

// GateAt is where the succession guarding the flow was declared.
func (f ObjectFlow) GateAt() Extent { return extentOf(f.Gate) }

// At is where the binding was declared.
func (b PinBinding) At() Extent {
	if b.Decl == nil {
		return Extent{}
	}
	return extentOf(b.Decl)
}

// OtherAt is where the binding's other end was written.
func (b PinBinding) OtherAt() Extent { return extentOf(b.Other) }

// PathNames are the names of the nodes the binding's own end steps through.
func (b PinBinding) PathNames() []string { return siteNames(b.Path) }

// OtherPathNames are the names of the nodes the binding's other end steps through.
func (b PinBinding) OtherPathNames() []string { return siteNames(b.OtherPath) }

// At is where the attribute was declared.
func (a Attribute) At() Extent { return extentOf(a.Node) }

// At is where the feature was declared.
func (f Feature) At() Extent { return extentOf(f.Node) }

// At is where the behavior was declared.
func (b StateBehavior) At() Extent { return extentOf(b.Node) }

// NodeNames are the names of the action nodes the behavior's blocks declare.
func (b StateBehavior) NodeNames() []string { return siteNames(b.Nodes) }

// At is where the transition was declared.
func (t *Transition) At() Extent { return extentOf(t.Decl) }

// At is where the entry transition was declared.
func (t *EntryTransition) At() Extent { return extentOf(t.Decl) }

// SubsetsAt is where the event the accept's payload subsets was written.
func (a Accept) SubsetsAt() Extent { return extentOf(a.SubsetsEvent) }

// SignalName is the signal type the accept names, as written.
func (a Accept) SignalName() string { return a.SignalType.Text() }

// StatementAt is where a statement was written, a send by its message; the zero
// Extent for one lowering supplied.
func StatementAt(s Statement) Extent {
	switch s := s.(type) {
	case Send:
		return extentOf(s.Message)
	case Assign:
		return extentOf(s.Node)
	case Declare:
		return extentOf(s.Node)
	case DeclareUsage:
		if s.Node == nil {
			return Extent{}
		}
		return extentOf(s.Node)
	case Block:
		return extentOf(s.Node)
	case Loop:
		return extentOf(s.Node)
	case If:
		return extentOf(s.Node)
	case Return:
		return extentOf(s.Node)
	case Effect:
		return extentOf(s.Node)
	case Unsupported:
		return extentOf(s.Node)
	}
	return Extent{}
}

// Trigger is what an accept or a transition waits for, as it was written.
type Trigger struct {
	// Kind is `accept`, `call`, `change` or `time`, else the label of what was written.
	Kind string
	Span source.Span
	// SignalType, Subsets and Payload are an accept trigger's.
	SignalType, Subsets, Payload string
	// Operation and Parameters are a call trigger's.
	Operation  string
	Parameters []string
	// Expr is where a change trigger's condition or a time trigger's duration was written.
	Expr Extent
	// Absolute reports a time trigger written `at` rather than `after`.
	Absolute bool
	// Signal and Called are the last segments of the signal type and of the
	// operation; Typed and Calls report that one was written.
	Signal, Called string
	Typed, Calls   bool
	// PayloadName is the payload's declared name; Subset is what the trigger
	// subsets, one name per link of a feature chain, and Subsetting reports
	// that it subsets something.
	PayloadName string
	Subset      []string
	Subsetting  bool
}

// endSegment is the last segment of a qualified name, "" for none.
func endSegment(name *ast.QualifiedName) string {
	if name == nil || len(name.Parts) == 0 {
		return ""
	}
	return name.Parts[len(name.Parts)-1].Text
}

// writtenName is a name as written, its segments joined by `::`.
func writtenName(node ast.Node) string {
	if qn := ast.AsQualifiedName(node); qn != nil {
		parts := make([]string, 0, len(qn.Parts))
		for _, part := range qn.Parts {
			parts = append(parts, part.Text)
		}
		return strings.Join(parts, "::")
	}
	return ast.SimpleName(node)
}

// writtenChain is the names a feature chain was written with, first link
// first; a name that is no chain is its only link.
func writtenChain(node ast.Node) []string {
	if chain, ok := node.(*ast.FeatureChainExpr); ok {
		return append(writtenChain(chain.Operand), writtenName(chain.Member))
	}
	return []string{writtenName(node)}
}

// triggerOf describes a trigger, false for none.
func triggerOf(node ast.Node) (Trigger, bool) {
	if absent(node) {
		return Trigger{}, false
	}
	form := Trigger{Span: node.Span()}
	switch t := node.(type) {
	case *ast.AcceptEvent:
		form.Kind = "accept"
		form.SignalType = t.SignalType.Text()
		if qn := ast.AsQualifiedName(t.Subsets); qn != nil {
			form.Subsets = qn.Text()
		} else {
			form.Subsets = FeaturePath(t.Subsets)
		}
		if t.Payload != nil {
			form.Payload, _ = ast.EffectiveName(t.Payload)
			form.PayloadName = t.Payload.Ident.Name
		}
		form.Typed, form.Signal = t.SignalType != nil, endSegment(t.SignalType)
		if t.Subsets != nil {
			form.Subsetting, form.Subset = true, writtenChain(t.Subsets)
		}
	case *ast.CallEvent:
		form.Kind = "call"
		form.Operation = t.Operation.Text()
		form.Calls, form.Called = t.Operation != nil, endSegment(t.Operation)
		for _, p := range t.Parameters {
			form.Parameters = append(form.Parameters, p.Text)
		}
	case *ast.ChangeEvent:
		form.Kind = "change"
		form.Expr = extentOf(t.Condition)
	case *ast.TimeEvent:
		form.Kind = "time"
		form.Expr = extentOf(t.Duration)
		form.Absolute = t.Absolute
	default:
		form.Kind = siteKind(node)
	}
	return form, true
}

// TriggerWritten describes the accept's trigger, false for an accept stating none.
func (a Accept) TriggerWritten() (Trigger, bool) { return triggerOf(a.Trigger) }

// TriggerWritten describes the transition's trigger, false for a completion transition.
func (t *Transition) TriggerWritten() (Trigger, bool) { return triggerOf(t.Trigger) }

// performedNames are the names a node performs or is typed by, as the runtime reads
// them: an invocation's callee, a `perform` target, a usage's typing or reference.
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

// performed resolves in scope the names node performs, leaving out those naming nothing.
func performed(node ast.Node, resolver *resolve.Resolver, scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, qn := range performedNames(node) {
		if sym, ok := resolver.ResolveQualified(scope, qn); ok && sym != nil {
			out = append(out, sym)
		}
	}
	return out
}

// PerformedBy are the behaviors the node behind id performs or is typed by, resolved in scope.
func (g *ActionGraph) PerformedBy(id program.NodeID, resolver *resolve.Resolver, scope *symbols.Scope) []*symbols.Symbol {
	return performed(g.Provenance.Node(id), resolver, scope)
}

// Performed are the behaviors the effect performs or is typed by, resolved in scope.
func (e Effect) Performed(resolver *resolve.Resolver, scope *symbols.Scope) []*symbols.Symbol {
	return performed(e.Node, resolver, scope)
}

// FootprintOf is the footprint of the node behind id, false for a node the graph has none for.
func (g *ActionGraph) FootprintOf(id program.NodeID) (Footprint, bool) {
	fp, ok := g.Footprints()[g.Provenance.Node(id)]
	return fp, ok
}

// ControlIDs are the nodes the footprint converges on, 0 for one the graph does not index.
func (g *ActionGraph) ControlIDs(fp Footprint) []program.NodeID { return g.nodeIDs(fp.Control) }
