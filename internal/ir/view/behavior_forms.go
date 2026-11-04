package view

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The kinds of the action nodes §8.2.3.17 gives a figure of their own: the
// keyword compartment heads each, and the node's expression follows it.
const (
	sendActionKind   = "send action"
	acceptActionKind = "accept action"
	assignKind       = "assign"
	loopKind         = "loop"
	ifKind           = "if"
)

// The kinds of a compound node's body compartments that hold an action flow
// (loop-body, then-body, else-body), drawn as a container nested in the node.
const (
	loopBodyKind = "loop body"
	thenBodyKind = "then body"
	elseBodyKind = "else body"
)

// The names of the compartments that head a nested flow: the action-flow
// compartment of an action, the state-transition compartment of a state.
const (
	actionFlowCompartment      = "action flow"
	stateTransitionCompartment = "state transition"
)

// statementRowKind is the kind of a compartment row that is a statement written
// out, which heads no row of its own.
const statementRowKind = "statement"

// statementKind reports whether a kind is a statement node's, whose keyword
// compartment heads the node whether or not it is named.
func statementKind(kind string) bool {
	switch kind {
	case sendActionKind, acceptActionKind, assignKind, loopKind, ifKind:
		return true
	}
	return false
}

// sectionKind reports whether a kind is a body compartment of a compound node,
// a container drawn as a compartment of the node holding it.
func sectionKind(kind string) bool {
	switch kind {
	case loopBodyKind, thenBodyKind, elseBodyKind, swimlaneKind:
		return true
	}
	return false
}

// statementNodeKind names an action usage by the one statement its body is —
// `action a accept x : X;`, `action b send S() to p;`, `action c assign v := 1;`,
// `action d while c { … }`, `action e if c { … }` — and "" for any other.
func statementNodeKind(node *ast.Usage, graph *lower.ActionGraph) string {
	if graph == nil {
		return ""
	}
	if _, ok := graph.Accepts[node]; ok {
		return acceptActionKind
	}
	body := graph.Bodies[node]
	if len(body) != 1 {
		return ""
	}
	switch body[0].(type) {
	case lower.Send:
		return sendActionKind
	case lower.Assign:
		return assignKind
	case lower.Loop:
		return loopKind
	case lower.If:
		return ifKind
	}
	return ""
}

// statementExpression is the expression the figure sets under a statement node's
// name compartment: the accept's payload (`reading : Reading`), the send's
// message and addressee (`Alarm() to sensor`), the assignment (`count := count + 1`).
func (r *Renderer) statementExpression(graph *lower.ActionGraph, node ast.Node, doc string) string {
	if accept, ok := graph.Accepts[node]; ok {
		return r.acceptExpression(doc, accept)
	}
	body := graph.Bodies[node]
	if len(body) != 1 {
		return ""
	}
	switch stmt := body[0].(type) {
	case lower.Send:
		return r.sendExpression(doc, stmt)
	case lower.Assign:
		return r.assignmentText(doc, body)
	}
	return ""
}

// acceptExpression is the accept-action-expression: the payload parameter as
// declared, `reading : Reading`, else the event the accept waits for, then the
// port it is accepted `via`.
func (r *Renderer) acceptExpression(doc string, accept lower.Accept) string {
	expression := r.acceptText(doc, accept)
	if accept.ParamName != "" && accept.SignalType != nil {
		expression = nameText(accept.ParamName) + " : " + expression
	}
	if accept.ViaPort != "" {
		expression = joinNonEmpty(expression, "via "+notationName(accept.ViaPort))
	}
	return expression
}

// sendExpression is the send-action-expression: the message, then `via` the
// port or `to` the receiver it is addressed to, as written.
func (r *Renderer) sendExpression(doc string, send lower.Send) string {
	parts := []string{}
	if message := r.messageText(doc, send.Message); message != "" {
		parts = append(parts, message)
	}
	if target := r.sendEnd(doc, send.Target, send.TargetExpr); target != "" {
		keyword := "to"
		if send.IsVia {
			keyword = "via"
		}
		parts = append(parts, keyword, target)
	}
	if receiver := r.sendEnd(doc, send.Receiver, send.ReceiverExpr); receiver != "" {
		parts = append(parts, "to", receiver)
	}
	return strings.Join(parts, " ")
}

// sendEnd is an addressee of a send: the name it was written as, else the
// expression as written.
func (r *Renderer) sendEnd(doc, name string, expr ast.Node) string {
	if name != "" {
		return notationName(name)
	}
	return r.nodeText(doc, expr)
}

// compoundBody is a body of a loop or conditional that is an action flow of its
// own, drawn as a section of the node.
type compoundBody struct {
	kind  string
	block lower.Block
}

// compoundCompartments are the compartments of a loop or conditional node, as
// the figures stack them: `while condition`, `loop body`, `until condition`;
// `for iterator`, `loop body`; `if condition`, `then body`, `else body`. A body
// of statements is listed in its compartment; one that is a flow is a section
// returned for the caller to draw the flow in.
func (r *Renderer) compoundCompartments(graph *lower.ActionGraph, node ast.Node, doc string, ids *nodeIDs) ([]Compartment, []compoundBody) {
	body := graph.Bodies[node]
	if len(body) != 1 {
		return nil, nil
	}
	var compartments []Compartment
	defer func() { numberRows(ids, compartments) }()
	var bodies []compoundBody
	expression := func(name string, expr ast.Node) {
		if expr != nil {
			compartments = append(compartments, Compartment{Name: name, Rows: statementRows(r.nodeText(doc, expr))})
		}
	}
	section := func(kind string, block *lower.Block) {
		if block == nil {
			return
		}
		if block.Graph != nil && len(block.Graph.Nodes) > 0 {
			bodies = append(bodies, compoundBody{kind: kind, block: *block})
			return
		}
		compartments = append(compartments, Compartment{Name: kind, Rows: r.statementListRows(doc, block.Statements)})
	}
	switch stmt := body[0].(type) {
	case lower.Loop:
		switch stmt.Kind {
		case ast.LoopFor:
			if iteration := r.iterationText(doc, stmt); iteration != "" {
				compartments = append(compartments, Compartment{Name: "for iterator", Rows: statementRows(iteration)})
			}
			section(loopBodyKind, &stmt.Body)
		case ast.LoopUntil:
			section(loopBodyKind, &stmt.Body)
			expression("until condition", stmt.Condition)
		default:
			expression("while condition", stmt.Condition)
			section(loopBodyKind, &stmt.Body)
			expression("until condition", stmt.Until)
		}
	case lower.If:
		expression("if condition", stmt.Condition)
		section(thenBodyKind, &stmt.Then)
		section(elseBodyKind, stmt.Else)
	}
	return compartments, bodies
}

// iterationText is a for loop's iteration-expression, `i in 1..3`.
func (r *Renderer) iterationText(doc string, loop lower.Loop) string {
	collection := r.nodeText(doc, loop.Collection)
	if loop.Variable == "" || collection == "" {
		return ""
	}
	return nameText(loop.Variable) + " in " + collection
}

// numberRows gives every compartment row an ID of its own, so that the view
// data and the canvas can tell the rows apart.
func numberRows(ids *nodeIDs, compartments []Compartment) {
	for _, compartment := range compartments {
		for _, row := range compartment.Rows {
			if row.ID == "" {
				row.ID = ids.take()
			}
		}
	}
}

// statementRows is the one row of a compartment holding an expression.
func statementRows(text string) []*Node {
	if text == "" {
		return nil
	}
	return []*Node{{Kind: statementRowKind, Text: text}}
}

// statementListRows lists a body of statements as compartment rows, each as
// written (action-body-textual), a nested block's statements in its place.
func (r *Renderer) statementListRows(doc string, statements []lower.Statement) []*Node {
	var rows []*Node
	for _, statement := range statements {
		if block, ok := statement.(lower.Block); ok {
			rows = append(rows, r.statementListRows(doc, block.Steps())...)
			continue
		}
		if text := r.statementText(doc, statement); text != "" {
			rows = append(rows, &Node{Kind: statementRowKind, Text: text})
		}
	}
	return rows
}

// statementText writes one statement as the notation does: a loop or
// conditional by its head alone, every other as written, without its `;`.
func (r *Renderer) statementText(doc string, statement lower.Statement) string {
	switch stmt := statement.(type) {
	case lower.Loop:
		switch stmt.Kind {
		case ast.LoopFor:
			return joinNonEmpty("for", r.iterationText(doc, stmt))
		case ast.LoopUntil:
			return joinNonEmpty("loop until", r.nodeText(doc, stmt.Condition))
		}
		return joinNonEmpty("while", r.nodeText(doc, stmt.Condition))
	case lower.If:
		return joinNonEmpty("if", r.nodeText(doc, stmt.Condition))
	case lower.Send:
		return joinNonEmpty("send", r.sendExpression(doc, stmt))
	case lower.Assign:
		return joinNonEmpty("assign", r.assignmentText(doc, []lower.Statement{stmt}))
	case lower.Effect:
		return joinNonEmpty(stmt.Kind.String(), r.nodeText(doc, stmt.TargetExpr))
	case lower.Return:
		return joinNonEmpty("return", r.nodeText(doc, stmt.Value))
	}
	return strings.TrimSuffix(strings.TrimSpace(r.nodeText(doc, statementNode(statement))), ";")
}

// statementNode is the declaration a statement was lowered from, nil for one
// that records none.
func statementNode(statement lower.Statement) ast.Node {
	switch stmt := statement.(type) {
	case lower.Declare:
		return stmt.Node
	case lower.DeclareUsage:
		return stmt.Node
	case lower.Assert:
		return stmt.Node
	case lower.Unsupported:
		return stmt.Node
	}
	return nil
}

// bodySection is the container a compound node's flow body is drawn in: a
// section of the node's box headed by the body compartment's name.
func (r *Renderer) bodySection(body compoundBody, doc string, ids *nodeIDs) *Node {
	return &Node{ID: ids.take(), Kind: body.kind}
}

// swimlaneKind is the kind of a swimlane: the dashed lane of a perform-actions
// view holding the nodes one performer performs (§8.2.3.17 swimlane).
const swimlaneKind = "swimlane"
