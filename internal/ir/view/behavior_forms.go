package view

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

// resultCompartment names the compartment holding a calculation's result expression.
const resultCompartment = "result"

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

// bodySectionKind reports whether a kind is a body compartment of a compound node,
// a container drawn as a compartment of the node holding it; a swimlane is none.
func bodySectionKind(kind string) bool {
	switch kind {
	case loopBodyKind, thenBodyKind, elseBodyKind:
		return true
	}
	return false
}

// dashedFrameKind reports whether a container of the kind is drawn with a
// dashed outline: an orthogonal region (§8.2.3.18) or a swimlane (§8.2.3.17).
func dashedFrameKind(kind string) bool {
	return kind == "region" || kind == swimlaneKind
}

// statementNodeKind names an action usage by the one statement its body is —
// `action a accept x : X;`, `action b send S() to p;`, `action c assign v := 1;`,
// `action d while c { … }`, `action e if c { … }` — and "" for any other.
func statementNodeKind(graph *lower.ActionGraph, node program.NodeID) string {
	if _, ok := graph.AcceptOf(node); ok {
		return acceptActionKind
	}
	body := graph.BodyOf(node)
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
func (r *Renderer) statementExpression(graph *lower.ActionGraph, node program.NodeID, doc string) string {
	if accept, ok := graph.AcceptOf(node); ok {
		return r.acceptExpression(doc, accept)
	}
	body := graph.BodyOf(node)
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
	if _, typed := accept.Signal(); accept.ParamName != "" && typed {
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
	if message := r.messageText(doc, send); message != "" {
		parts = append(parts, message)
	}
	if target := r.sendEnd(doc, send.Target, send.TargetAt()); target != "" {
		keyword := "to"
		if send.IsVia {
			keyword = "via"
		}
		parts = append(parts, keyword, target)
	}
	if receiver := r.sendEnd(doc, send.Receiver, send.ReceiverAt()); receiver != "" {
		parts = append(parts, "to", receiver)
	}
	return strings.Join(parts, " ")
}

// sendEnd is an addressee of a send: the name it was written as, else the
// expression as written.
func (r *Renderer) sendEnd(doc, name string, expr lower.Extent) string {
	if name != "" {
		return notationName(name)
	}
	return r.extentText(doc, expr)
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
func (r *Renderer) compoundCompartments(graph *lower.ActionGraph, node program.NodeID, doc string, ids *nodeIDs) ([]Compartment, []compoundBody) {
	body := graph.BodyOf(node)
	if len(body) != 1 {
		return nil, nil
	}
	var compartments []Compartment
	defer func() { numberRows(ids, compartments) }()
	var bodies []compoundBody
	expression := func(name string, expr lower.Extent) {
		if expr.Written {
			compartments = append(compartments, Compartment{Name: name, Rows: statementRows(r.extentText(doc, expr))})
		}
	}
	section := func(kind string, block *lower.Block) {
		if block == nil {
			return
		}
		if block.Flows() {
			bodies = append(bodies, compoundBody{kind: kind, block: *block})
			return
		}
		compartments = append(compartments, Compartment{Name: kind, Rows: r.statementListRows(doc, block.Statements)})
	}
	switch stmt := body[0].(type) {
	case lower.Loop:
		switch {
		case stmt.For():
			if iteration := r.iterationText(doc, stmt); iteration != "" {
				compartments = append(compartments, Compartment{Name: "for iterator", Rows: statementRows(iteration)})
			}
			section(loopBodyKind, &stmt.Body)
		case stmt.PostTested():
			section(loopBodyKind, &stmt.Body)
			expression("until condition", stmt.ConditionAt())
		default:
			expression("while condition", stmt.ConditionAt())
			section(loopBodyKind, &stmt.Body)
			expression("until condition", stmt.UntilAt())
		}
	case lower.If:
		expression("if condition", stmt.ConditionAt())
		section(thenBodyKind, &stmt.Then)
		section(elseBodyKind, stmt.Else)
	}
	return compartments, bodies
}

// iterationText is a for loop's iteration-expression, `i in 1..3`.
func (r *Renderer) iterationText(doc string, loop lower.Loop) string {
	collection := r.extentText(doc, loop.CollectionAt())
	if !loop.Bound || collection == "" {
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

// resultCompartments is a calculation's `result` compartment (SysML v2 §8.2.3.19
// result-compartment): the result expression its own body states, as written.
func (r *Renderer) resultCompartments(calc *symbols.Symbol, ids *nodeIDs) []Compartment {
	var rows []*Node
	for _, expr := range semantics.OwnedResultExpressions(calc) {
		rows = append(rows, statementRows(r.nodeText(calc.DocName, expr.Node))...)
	}
	if len(rows) == 0 {
		return nil
	}
	compartments := []Compartment{{Name: resultCompartment, Rows: rows}}
	numberRows(ids, compartments)
	return compartments
}

// statementRows is the one row of a compartment holding an expression.
func statementRows(text string) []*Node {
	if text == "" {
		return nil
	}
	return []*Node{{Kind: statementRowKind, Text: text}}
}

// statementListRows lists a body of statements as compartment rows, each as
// written (action-body-textual): a nested loop or conditional by its head with
// its body's rows indented under it, a nested block's statements in its place.
func (r *Renderer) statementListRows(doc string, statements []lower.Statement) []*Node {
	return r.indentedStatementRows(doc, statements, "")
}

// statementIndent is the indent of a nested body's rows under its head.
const statementIndent = "  "

func (r *Renderer) indentedStatementRows(doc string, statements []lower.Statement, indent string) []*Node {
	var rows []*Node
	row := func(text string) {
		if text != "" {
			rows = append(rows, &Node{Kind: statementRowKind, Text: indent + text})
		}
	}
	body := func(block *lower.Block) {
		if block != nil {
			rows = append(rows, r.indentedStatementRows(doc, block.Steps(), indent+statementIndent)...)
		}
	}
	for _, statement := range statements {
		switch stmt := statement.(type) {
		case lower.Block:
			rows = append(rows, r.indentedStatementRows(doc, stmt.Steps(), indent)...)
		case lower.Loop:
			if stmt.PostTested() {
				row("loop")
				body(&stmt.Body)
				if at := stmt.ConditionAt(); at.Written { // a `loop { }` written without `until` has none
					row(joinNonEmpty("until", r.extentText(doc, at)))
				}
				continue
			}
			row(r.statementText(doc, stmt))
			body(&stmt.Body)
			if at := stmt.UntilAt(); at.Written {
				row(joinNonEmpty("until", r.extentText(doc, at)))
			}
		case lower.If:
			row(r.statementText(doc, stmt))
			body(&stmt.Then)
			if stmt.Else != nil {
				row("else")
				body(stmt.Else)
			}
		default:
			row(r.statementText(doc, statement))
		}
	}
	return rows
}

// statementText writes one statement's head as the notation does: a loop or
// conditional by its head (its body rows follow it), every other as written, without its `;`.
func (r *Renderer) statementText(doc string, statement lower.Statement) string {
	switch stmt := statement.(type) {
	case lower.Loop:
		switch {
		case stmt.For():
			return joinNonEmpty("for", r.iterationText(doc, stmt))
		case stmt.PostTested():
			return joinNonEmpty("loop until", r.extentText(doc, stmt.ConditionAt()))
		}
		return joinNonEmpty("while", r.extentText(doc, stmt.ConditionAt()))
	case lower.If:
		return joinNonEmpty("if", r.extentText(doc, stmt.ConditionAt()))
	case lower.Send:
		return joinNonEmpty("send", r.sendExpression(doc, stmt))
	case lower.Assign:
		return joinNonEmpty("assign", r.assignmentText(doc, []lower.Statement{stmt}))
	case lower.Effect:
		return joinNonEmpty(stmt.Kind.String(), r.extentText(doc, stmt.TargetAt()))
	case lower.Return:
		return joinNonEmpty("return", r.extentText(doc, stmt.ValueAt()))
	}
	return strings.TrimSuffix(strings.TrimSpace(r.extentText(doc, statementAt(statement))), ";")
}

// statementAt is where a statement written as a declaration stands, the zero
// Extent for any other.
func statementAt(statement lower.Statement) lower.Extent {
	switch stmt := statement.(type) {
	case lower.Declare, lower.DeclareUsage, lower.Unsupported:
		return lower.StatementAt(stmt)
	case lower.Assert:
		return stmt.At()
	}
	return lower.Extent{}
}

// bodySection is the container a compound node's flow body is drawn in: a
// section of the node's box headed by the body compartment's name.
func (r *Renderer) bodySection(body compoundBody, doc string, ids *nodeIDs) *Node {
	return &Node{ID: ids.take(), Kind: body.kind}
}

// swimlaneKind is the kind of a swimlane: the dashed lane of a perform-actions
// view holding the nodes one performer performs (§8.2.3.17 swimlane).
const swimlaneKind = "swimlane"
