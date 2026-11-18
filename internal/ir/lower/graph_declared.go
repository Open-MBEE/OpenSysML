package lower

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The accessors below hand a reader that attaches to declarations (a view, a
// check) the declaration behind an id or record, and the document it is in.

// StateDecl is the declaration the state was lowered from, nil for no state.
func (g *StateGraph) StateDecl(id program.StateID) ast.Node {
	state := g.stateOf(id)
	if state == nil {
		return nil
	}
	return g.DeclOf(state)
}

// RegionDecl is the declaration the region was built from, nil for no region.
func (g *StateGraph) RegionDecl(id program.RegionID) ast.Node {
	region := g.Provenance.Region(id)
	if region == nil {
		return nil
	}
	return g.RegionDeclOf(region)
}

// VertexDoc is the document the vertex was written in, the machine's for no vertex.
func (g *StateGraph) VertexDoc(ref program.VertexRef) string {
	return g.DocOf(g.vertexDecl(ref))
}

// RegionDoc is the document the region was written in, the machine's for no region.
func (g *StateGraph) RegionDoc(id program.RegionID) string {
	return g.DocOf(g.Provenance.Region(id))
}

// TransitionDoc is the document the transition was written in, the general's where inherited.
func (g *StateGraph) TransitionDoc(t *Transition) string { return g.DocOf(t.Decl) }

// EntryDoc is the document the entry transition was written in, the general's where inherited.
func (g *StateGraph) EntryDoc(t *EntryTransition) string { return g.DocOf(t.Decl) }

// Completing reports whether the state is the `done` vertex of the body naming it.
func (g *StateGraph) Completing(id program.StateID) bool {
	return g.Completes(g.Provenance.State(id))
}

// StartOfState are the transitions out of the entry action of the state's body,
// those of the region it stands for when it does; 0 is the machine's own body.
func (g *StateGraph) StartOfState(id program.StateID) []*EntryTransition {
	if id == 0 {
		return g.StartOf(nil)
	}
	state := g.stateOf(id)
	if state == nil {
		return nil
	}
	return g.StartOf(state)
}

// StartsBody reports whether the body declaring the state starts in it whatever its guards say.
func (g *StateGraph) StartsBody(id program.StateID) bool {
	state := g.Provenance.State(id)
	if state == nil {
		return false
	}
	var owner ast.Node
	if region := g.RegionOf[state]; region != nil {
		owner = region
	} else if parent := g.ParentState[state]; parent != nil {
		owner = parent
	}
	return g.UnconditionalStart(owner) == state
}

// Declared is the declaration the transition was lowered from.
func (t *Transition) Declared() ast.Node { return t.Decl }

// Declared is the declaration the entry transition was lowered from.
func (t *EntryTransition) Declared() ast.Node { return t.Decl }

// Declared is the declaration the behavior was lowered from.
func (b StateBehavior) Declared() ast.Node { return b.Node }

// EndNames are the names of the vertices the transition leaves and reaches, "" for an unnamed one.
func (t *Transition) EndNames() (source, target string) {
	return siteName(t.Source), siteName(t.Target)
}

// TargetVertex is the vertex the entry transition reaches first, its
// pseudostate when it runs through one, and that vertex's name.
func (t *EntryTransition) TargetVertex() (program.VertexRef, string) {
	if t.Via != nil {
		return program.PseudoVertex(t.ViaID), t.Via.Name
	}
	return program.StateVertex(t.TargetID), siteName(t.Target)
}

// GuardAt is where the transition's guard was written, the zero Extent for none.
func (t *Transition) GuardAt() Extent { return extentOf(t.Guard) }

// GuardAt is where the entry transition's guard was written, the zero Extent for none.
func (t *EntryTransition) GuardAt() Extent { return extentOf(t.Guard) }

// Signal is the last segment of the signal type the accept names, false for an accept naming none.
func (a Accept) Signal() (string, bool) { return endSegment(a.SignalType), a.SignalType != nil }

// Declared is the declaration the succession is drawn from: the succession flow
// gating it, else its own.
func (e ActionEdge) Declared() ast.Node {
	if e.Gate != nil {
		return e.Gate
	}
	return e.Decl
}

// Declared is the declaration the object flow was lowered from.
func (f ObjectFlow) Declared() ast.Node { return f.Decl }

// Declared is the usage the binding was lowered from.
func (b PinBinding) Declared() *ast.Usage { return b.Decl }

// PathDecls are the declarations Path steps through into nested flows, which
// this graph does not number.
func (b PinBinding) PathDecls() []ast.Node { return b.Path }

// OtherPathDecls are the declarations OtherPath steps through into nested flows.
func (b PinBinding) OtherPathDecls() []ast.Node { return b.OtherPath }

// Gated reports whether a succession flow gates the succession.
func (e ActionEdge) Gated() bool { return e.Gate != nil }

// AtNode reports whether the binding's own end is at a node rather than at the action itself.
func (b PinBinding) AtNode() bool { return b.Node != nil }

// OtherAtNode reports whether the binding's other end is at a node.
func (b PinBinding) OtherAtNode() bool { return b.OtherNode != nil }

// Succession is the succession's own declaration, whether or not a flow gates it.
func (e ActionEdge) Succession() ast.Node { return e.Decl }

// Doc is the document the inherited declaration was written in, "" outside any.
func (in Inherited) Doc() string { return symbols.DocNameOf(in.Body) }

// Declared is the declaration the graph took content from.
func (in Inherited) Declared() ast.Node { return in.Decl }

// Declared is the declaration the feature was lowered from.
func (f Feature) Declared() ast.Node { return f.Node }

// ValueAt is where the feature's value was written, the zero Extent for none.
func (f Feature) ValueAt() Extent { return extentOf(f.Value) }

// Bound reports whether the feature states a value.
func (f Feature) Bound() bool { return f.Value != nil }

// ValueName is the last segment of the name the feature's value references,
// false for a value that is no reference.
func (f Feature) ValueName() (string, bool) {
	qn := ast.AsQualifiedName(f.Value)
	if qn == nil || len(qn.Parts) == 0 {
		return "", false
	}
	return qn.Parts[len(qn.Parts)-1].Text, true
}

// ValueLiteral is the feature's literal value as the notation writes it, ""
// for a value that is no literal.
func (f Feature) ValueLiteral() string {
	switch e := f.Value.(type) {
	case *ast.LiteralBool:
		return strconv.FormatBool(e.Value)
	case *ast.LiteralString:
		return e.Value
	case *ast.LiteralInteger:
		return e.Value
	case *ast.LiteralReal:
		return e.Value
	case *ast.LiteralInfinity:
		return "*"
	case *ast.NullExpr:
		return "null"
	}
	return ""
}

// GuardAt is where the succession's guard was written, the zero Extent for none.
func (e ActionEdge) GuardAt() Extent { return extentOf(e.Guard) }

// WeightAt is where the succession's probability was written, false for an unweighted one.
func (e ActionEdge) WeightAt() (Extent, bool) {
	if e.Probability == nil {
		return Extent{}, false
	}
	return extentOf(e.Probability.Expr), true
}

// MessageAt is where the send's message was written.
func (s Send) MessageAt() Extent { return extentOf(s.Message) }

// Constructs is the last segment of the type the send's message constructs,
// false for a message that is any other expression.
func (s Send) Constructs() (string, bool) {
	switch m := s.Message.(type) {
	case *ast.ConstructorExpr:
		return endSegment(m.Type), true
	case *ast.InvocationExpr:
		if m.Operand == nil {
			return endSegment(m.Type), true
		}
	}
	return "", false
}

// TargetAt is where the send's `via` or `to` addressee was written, the zero Extent for none.
func (s Send) TargetAt() Extent { return extentOf(s.TargetExpr) }

// ReceiverAt is where the `to` receiver of a routed send was written, the zero Extent for none.
func (s Send) ReceiverAt() Extent { return extentOf(s.ReceiverExpr) }

// Sides are where the assignment's target and value were written, false for an
// assignment lowered from no assignment statement.
func (a Assign) Sides() (target, value Extent, ok bool) {
	stmt, ok := a.Node.(*ast.AssignmentActionNode)
	if !ok {
		return Extent{}, Extent{}, false
	}
	return extentOf(stmt.Target), extentOf(stmt.Value), true
}

// ConditionAt is where the loop's condition was written, the zero Extent for none.
func (l Loop) ConditionAt() Extent { return extentOf(l.Condition) }

// UntilAt is where a `while` loop's `until` condition was written, the zero Extent for none.
func (l Loop) UntilAt() Extent { return extentOf(l.Until) }

// CollectionAt is where the collection a `for` loop iterates was written.
func (l Loop) CollectionAt() Extent { return extentOf(l.Collection) }

// ConditionAt is where the conditional's condition was written.
func (i If) ConditionAt() Extent { return extentOf(i.Condition) }

// TargetAt is where the effect's target was written.
func (e Effect) TargetAt() Extent { return extentOf(e.TargetExpr) }

// ValueAt is where the returned value was written, the zero Extent for none.
func (r Return) ValueAt() Extent { return extentOf(r.Value) }

// Flows reports whether the block's statements are a flow of their own.
func (block Block) Flows() bool { return block.Graph != nil && len(block.Graph.Nodes) > 0 }

// Declared is the loop or branch the block belongs to.
func (block Block) Declared() ast.Node { return block.Node }

// At is where the assertion was written, the zero Extent for one lowering supplied.
func (a Assert) At() Extent {
	if a.Node == nil {
		return Extent{}
	}
	return extentOf(a.Node)
}

// Syntax is the returned expression as written, nil for a return of none; a
// reader compiling the expression itself (a query plan) starts from it.
func (r Return) Syntax() ast.Node { return r.Value }

// Syntax is the end's expression as written.
func (e BindingEnd) Syntax() ast.Node { return e.Expr }

// Declared is the connector the binding was lowered from.
func (b Binding) Declared() ast.Node { return b.Decl }
