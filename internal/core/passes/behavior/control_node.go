package behavior

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const controlNodeSource = "control-node"

// CodeControlNodeOwner marks a control node declared outside an action
// definition or usage (SysML v2 8.3.17.6 validateControlNodeOwningType).
const CodeControlNodeOwner = "control-node-owner"

// Diagnostic codes for the bounds SysML v2 8.3.17 places on the successions of
// each control-node kind, one per validation constraint.
const (
	// CodeControlNodeIncomingMultiplicity: a succession into a control node
	// writes a target multiplicity other than 1..1
	// (validateControlNodeIncomingSuccessions).
	CodeControlNodeIncomingMultiplicity = "control-node-incoming-multiplicity"
	// CodeControlNodeOutgoingMultiplicity: a succession out of a control node
	// writes a source multiplicity other than 1..1
	// (validateControlNodeOutgoingSuccessions).
	CodeControlNodeOutgoingMultiplicity = "control-node-outgoing-multiplicity"
	// CodeForkIncomingSuccessions: a fork node has more than one incoming
	// succession (validateForkNodeIncomingSuccessions).
	CodeForkIncomingSuccessions = "fork-incoming-successions"
	// CodeJoinOutgoingSuccessions: a join node has more than one outgoing
	// succession (validateJoinNodeOutgoingSuccessions).
	CodeJoinOutgoingSuccessions = "join-outgoing-successions"
	// CodeMergeIncomingMultiplicity: a succession into a merge node writes a
	// source multiplicity other than 0..1 (validateMergeNodeIncomingSuccessions).
	CodeMergeIncomingMultiplicity = "merge-incoming-multiplicity"
	// CodeMergeOutgoingSuccessions: a merge node has more than one outgoing
	// succession (validateMergeNodeOutgoingSuccessions).
	CodeMergeOutgoingSuccessions = "merge-outgoing-successions"
	// CodeDecisionIncomingSuccessions: a decision node has more than one
	// incoming succession (validateDecisionNodeIncomingSuccessions).
	CodeDecisionIncomingSuccessions = "decision-incoming-successions"
	// CodeDecisionOutgoingMultiplicity: a succession out of a decision node
	// writes a target multiplicity other than 0..1
	// (validateDecisionNodeOutgoingSuccessions).
	CodeDecisionOutgoingMultiplicity = "decision-outgoing-multiplicity"
)

// ControlNodeSuccessionPass checks the successions of fork, join, merge, and
// decision nodes against the bounds SysML v2 7.17.3 places on each kind, and that
// every control node is owned by an action definition or usage.
type ControlNodeSuccessionPass struct{}

func (ControlNodeSuccessionPass) Level() kit.PassLevel { return kit.LevelConstraint }

// ElementScoped: each control node is its own subject.
func (ControlNodeSuccessionPass) ElementScoped() { /* marker: per-element gating */ }

func (ControlNodeSuccessionPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	c := &controlNodeChecker{ctx: ctx, model: ctx.Model()}
	c.walk(scope, nil, root.Members)
	return c.diags
}

type controlNodeChecker struct {
	ctx   *kit.Context
	model *semantics.Model
	diags []diag.Diagnostic
}

func (c *controlNodeChecker) walk(scope *symbols.Scope, owner ast.Node, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, owner, kit.UnwrapMembership(member))
	}
}

// walkNode checks the control nodes and successions of every body shape, with
// owner the declaration whose body decl is a member of.
func (c *controlNodeChecker) walkNode(scope *symbols.Scope, owner, decl ast.Node) {
	child := kit.BodyScope(scope, decl)
	switch n := decl.(type) {
	case *ast.Package:
		c.walk(child, n, n.Members)
	case *ast.Namespace:
		c.walk(child, n, n.Members)
	case *ast.Definition:
		c.checkAction(scope, n, n.Members)
		c.walk(child, n, n.Members)
	case *ast.Usage:
		c.checkAction(scope, n, n.Members)
		c.walk(child, n, n.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.checkOwner(n, owner)
		c.checkAction(scope, n, ast.NodeBodyMembers(n))
		c.walk(child, n, ast.NodeBodyMembers(n))
	case *ast.InitialNode:
		c.checkAction(scope, n, n.Members)
		c.walk(child, n, n.Members)
	case *ast.SuccessionEdge:
		c.walk(child, n, n.Members)
	case *ast.SubjectMember:
		c.walk(child, n, n.Body)
	case *ast.ConstraintMember:
		c.walk(child, n, n.Body)
	case *ast.AssumeMember:
		c.walk(child, n, n.Body)
	case *ast.RequireMember:
		c.walk(child, n, n.Body)
	case *ast.EntryMember:
		c.checkBlock(child, n.Actions)
		c.walk(child, n, n.Actions)
	case *ast.DoMember:
		c.checkBlock(child, n.Actions)
		c.walk(child, n, n.Actions)
	case *ast.ExitMember:
		c.checkBlock(child, n.Actions)
		c.walk(child, n, n.Actions)
	case *ast.StateNode:
		c.walk(child, n, n.Entry)
		c.walk(child, n, n.Do)
		c.walk(child, n, n.Exit)
		c.walk(child, n, n.Substates)
		for _, region := range n.Regions {
			c.walkNode(child, n, region)
		}
	case *ast.StateRegion:
		c.walk(child, n, n.States)
	case *ast.TransitionMember:
		// Effect and body members are built in the trigger scope when the
		// trigger declares parameters.
		body := symbols.TriggerScope(scope, n)
		c.checkAction(scope, n, n.Members)
		c.checkBlock(body, n.Effect)
		c.walk(body, n, n.Effect)
		c.walk(body, n, n.Members)
	case *ast.SendStatement:
		c.checkAction(scope, n, n.Members)
		c.walk(child, n, n.Members)
	case *ast.WhileLoopActionNode:
		c.checkBlock(child, n.Body)
		c.walk(child, n, n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			c.walkNode(child, n, branch)
		}
	case *ast.IfBranchNode:
		c.checkBlock(child, n.Body)
		c.walk(child, n, n.Body)
	}
}

// checkOwner reports a control node whose owner is not an action definition or
// usage.
func (c *controlNodeChecker) checkOwner(node, owner ast.Node) {
	if actionOwner(owner) || c.ctx.DownstreamSpan(declarationHead(node)) {
		return
	}
	where := "outside any action"
	if text := declarationText(owner); text != "" {
		where = "in " + text + ", which is not an action"
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     controlNodeSpan(node),
		Message: fmt.Sprintf("%s is declared %s; declare it in the body of an action definition or usage",
			controlNodeText(node), where),
		Code:   CodeControlNodeOwner,
		Source: controlNodeSource,
	})
}

// actionOwner reports whether owner is an action declaration or an
// entry/do/exit block (an anonymous action usage).
func actionOwner(owner ast.Node) bool {
	switch owner.(type) {
	case *ast.EntryMember, *ast.DoMember, *ast.ExitMember:
		return true
	}
	return semantics.ActionDeclaration(owner)
}

// checkAction checks an action's control nodes against the successions it
// declares or inherits; an action without a symbol is checked as a block.
func (c *controlNodeChecker) checkAction(scope *symbols.Scope, decl ast.Node, body []ast.Node) {
	if !semantics.ActionDeclaration(decl) {
		return
	}
	if child := scope.ChildFor(decl); child != nil && child.Owner() != nil {
		sym := child.Owner()
		c.check(sym, c.model.ActionSuccessions(sym))
		return
	}
	c.checkBlock(kit.BodyScope(scope, decl), body)
}

// checkBlock checks the control nodes of a symbol-less action body: a loop or
// branch body, an entry/do/exit block, an anonymous transition or send.
func (c *controlNodeChecker) checkBlock(scope *symbols.Scope, body []ast.Node) {
	c.check(nil, c.model.DeclaredSuccessions(scope, nil, body))
}

// controlNodeFlows are the successions into and out of one control node.
type controlNodeFlows struct {
	node ast.Node
	// local is whether node is declared in the document being checked.
	local    bool
	incoming []semantics.ActionSuccession
	outgoing []semantics.ActionSuccession
}

// check applies each kind's bounds to every control node the successions reach.
// It reports only what owner's own body contributes — the node or an offending
// succession — so a violation inherited whole is reported once, at its source.
func (c *controlNodeChecker) check(owner *symbols.Symbol, succs []semantics.ActionSuccession) {
	var order []ast.Node
	flows := map[ast.Node]*controlNodeFlows{}
	attach := func(end semantics.ActionSuccessionEnd, s semantics.ActionSuccession, incoming bool) {
		node := end.Node
		if !isControlNode(node) {
			return
		}
		f := flows[node]
		if f == nil {
			f = &controlNodeFlows{node: node, local: c.localEnd(s, end)}
			flows[node] = f
			order = append(order, node)
		}
		if incoming {
			f.incoming = append(f.incoming, s)
		} else {
			f.outgoing = append(f.outgoing, s)
		}
	}
	// Each end rests on its own reference alone: a fault elsewhere in the
	// succession — its guard, effect or body — leaves the end sound.
	for _, s := range succs {
		if !c.unsoundEnd(s, s.Target) {
			attach(s.Target, s, true)
		}
		if !c.unsoundEnd(s, s.Source) {
			attach(s.Source, s, false)
		}
	}
	for _, node := range order {
		f := flows[node]
		if f.local && c.ctx.DownstreamSpan(declarationHead(node)) {
			continue
		}
		own := owner == nil || c.declares(owner, node)
		for _, s := range f.incoming {
			c.checkEndMultiplicity(owner, own, node, s, s.Target, endRule{1, 1, "into", "target",
				CodeControlNodeIncomingMultiplicity})
		}
		for _, s := range f.outgoing {
			c.checkEndMultiplicity(owner, own, node, s, s.Source, endRule{1, 1, "out of", "source",
				CodeControlNodeOutgoingMultiplicity})
		}
		switch node.(type) {
		case *ast.ForkNode:
			c.checkCount(owner, own, node, f.incoming, "incoming",
				"merge or join the flows before the fork", CodeForkIncomingSuccessions)
		case *ast.DecisionNode:
			c.checkCount(owner, own, node, f.incoming, "incoming",
				"merge the flows before the decision", CodeDecisionIncomingSuccessions)
			for _, s := range f.outgoing {
				c.checkEndMultiplicity(owner, own, node, s, s.Target, endRule{0, 1, "out of", "target",
					CodeDecisionOutgoingMultiplicity})
			}
		case *ast.JoinNode:
			c.checkCount(owner, own, node, f.outgoing, "outgoing",
				"follow the join with a fork or decision node to branch", CodeJoinOutgoingSuccessions)
		case *ast.MergeNode:
			c.checkCount(owner, own, node, f.outgoing, "outgoing",
				"follow the merge with a fork or decision node to branch", CodeMergeOutgoingSuccessions)
			for _, s := range f.incoming {
				c.checkEndMultiplicity(owner, own, node, s, s.Source, endRule{0, 1, "into", "source",
					CodeMergeIncomingMultiplicity})
			}
		}
	}
}

// unsoundEnd reports an end whose reference carries a lower-tier fault. The
// faults are spans in the document being checked, so only an end written there
// can carry one.
func (c *controlNodeChecker) unsoundEnd(s semantics.ActionSuccession, end semantics.ActionSuccessionEnd) bool {
	return end.Span.Len > 0 && c.local(s.Owner) && c.ctx.DownstreamSpan(end.Span)
}

// localEnd reports whether the node end attaches to is declared in the document
// being checked: where its symbol is, or the succession's owner for a
// positional end.
func (c *controlNodeChecker) localEnd(s semantics.ActionSuccession, end semantics.ActionSuccessionEnd) bool {
	if end.Symbol != nil {
		return c.local(end.Symbol)
	}
	return c.local(s.Owner)
}

// local reports whether sym is declared in the document being checked; a nil
// sym stands for a body-local block of it.
func (c *controlNodeChecker) local(sym *symbols.Symbol) bool {
	if sym == nil {
		return true
	}
	doc := symbols.DocNameOf(sym.OwnerScope)
	if doc == "" {
		doc = sym.DocName
	}
	return doc == "" || doc == c.ctx.Name
}

// declarationHead is the span of a node up to its body: what a rule about the
// node itself, rather than about its members, rests on.
func declarationHead(node ast.Node) source.Span {
	span := node.Span()
	if body := ast.NodeBodyMembers(node); len(body) > 0 {
		span.Len = body[0].Span().Offset - span.Offset
	}
	return span
}

// checkCount reports a node with more than one succession on the bounded side.
// It is reported at the node when owner declares it, else at the last succession
// owner declares, which is what pushed the count over.
func (c *controlNodeChecker) checkCount(owner *symbols.Symbol, own bool, node ast.Node,
	succs []semantics.ActionSuccession, side, fix, code string) {
	if len(succs) <= 1 {
		return
	}
	span, ok := controlNodeSpan(node), own
	if !own {
		for _, s := range succs {
			if s.Owner == owner {
				span, ok = s.Decl.Span(), true
			}
		}
	}
	if !ok {
		return
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message: fmt.Sprintf("%s has %d %s successions; a %s node may have at most one — %s",
			controlNodeText(node), len(succs), side, controlNodeKind(node), fix),
		Code:   code,
		Source: controlNodeSource,
	})
}

// checkEndMultiplicity reports a succession end that writes a multiplicity other
// than lower..upper. An end that writes none is implied to have the required
// one (SysML v2 7.17.3: the rules hold in the abstract syntax even when the
// notation does not show them), so only a written multiplicity can be wrong.
type endRule struct {
	lower, upper int64
	direction    string
	endName      string
	code         string
}

func (c *controlNodeChecker) checkEndMultiplicity(owner *symbols.Symbol, own bool, node ast.Node,
	s semantics.ActionSuccession, end semantics.ActionSuccessionEnd, rule endRule) {
	lower, upper, direction, endName, code := rule.lower, rule.upper, rule.direction, rule.endName, rule.code
	if end.Multiplicity == nil || (!own && s.Owner != owner) || c.ctx.DownstreamOfFailure(end.Multiplicity) {
		return
	}
	r, ok := c.model.RangeOf(end.Multiplicity)
	if !ok {
		return
	}
	holds, ok := r.HasBounds(lower, upper)
	if !ok || holds {
		return
	}
	want := semantics.Range{
		Lower: semantics.Bound{Value: lower, Known: true},
		Upper: semantics.Bound{Value: upper, Known: true},
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     end.Multiplicity.Span(),
		Message: fmt.Sprintf("succession %s %s has %s multiplicity %s; successions %s a %s node must have %s multiplicity %s",
			direction, controlNodeText(node), endName, r.Text(), direction, controlNodeKind(node), endName, want.Text()),
		Code:   code,
		Source: controlNodeSource,
	})
}

// declares reports whether node is declared in owner's own body.
func (c *controlNodeChecker) declares(owner *symbols.Symbol, node ast.Node) bool {
	if owner == nil || owner.Scope == nil {
		return false
	}
	for _, member := range owner.Scope.AllMembers() {
		if member.Decl == node {
			return true
		}
	}
	return false
}

func isControlNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		return true
	}
	return false
}

// controlNodeKind is the kind of a control node as the spec names it.
func controlNodeKind(node ast.Node) string {
	switch node.(type) {
	case *ast.ForkNode:
		return "fork"
	case *ast.JoinNode:
		return "join"
	case *ast.MergeNode:
		return "merge"
	case *ast.DecisionNode:
		return "decision"
	}
	return ""
}

// controlNodeName is a control node's keyword, name, and name span as written.
func controlNodeName(node ast.Node) (keyword, name string, span source.Span) {
	switch n := node.(type) {
	case *ast.ForkNode:
		return "fork", n.Name, n.NameSpan
	case *ast.JoinNode:
		return "join", n.Name, n.NameSpan
	case *ast.MergeNode:
		return "merge", n.Name, n.NameSpan
	case *ast.DecisionNode:
		return "decide", n.Name, n.NameSpan
	}
	return "", "", source.Span{}
}

// controlNodeText names a control node as written: `fork f`, or `unnamed fork`.
func controlNodeText(node ast.Node) string {
	keyword, name, _ := controlNodeName(node)
	if name == "" {
		return "unnamed " + keyword
	}
	return keyword + " " + name
}

func controlNodeSpan(node ast.Node) source.Span {
	if _, _, span := controlNodeName(node); span.Len > 0 {
		return span
	}
	return node.Span()
}

// declarationText names a definition, usage, package, namespace, or succession
// as written, or "" for any other node.
func declarationText(decl ast.Node) string {
	switch d := decl.(type) {
	case *ast.Definition:
		return withName(d.Keyword+" def", d.Ident.Name)
	case *ast.Usage:
		return withName(d.Keyword, d.Ident.Name)
	case *ast.Package:
		return withName("package", d.Ident.Name)
	case *ast.Namespace:
		return withName("namespace", d.Ident.Name)
	case *ast.InitialNode, *ast.SuccessionEdge:
		return "the body of a succession"
	case *ast.ConstraintMember:
		return withName("constraint", d.Name)
	case *ast.AssumeMember:
		return withName("constraint", d.Ident.Name)
	case *ast.RequireMember:
		return withName("constraint", d.Ident.Name)
	}
	return ""
}

func withName(keyword, name string) string {
	if name == "" {
		return "an unnamed " + keyword
	}
	return keyword + " " + name
}
