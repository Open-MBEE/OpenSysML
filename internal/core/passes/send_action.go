package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const sendActionSource = "send-action"

// Diagnostic codes of the send-action rules.
const (
	CodeSendPayloadMissing        = "send-payload-missing"
	CodeSendToPort                = "send-to-port"
	CodeSendReceiverNotOccurrence = "send-receiver-not-occurrence"
	CodeSendSenderNotOccurrence   = "send-sender-not-occurrence"
)

// SendActionPass types the payload, `via` and `to` arguments of every send and
// checks the SysML v2 SendActionUsage constraints on them.
type SendActionPass struct{}

// Level reports that this pass consumes name-resolution results.
func (SendActionPass) Level() PassLevel { return LevelType }

// ElementScoped lets each send argument gate independently on lower failures.
func (SendActionPass) ElementScoped() { /* marker: per-element gating */ }

// Run checks every send in the document.
func (SendActionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	expr := &exprChecker{resolver: ctx.Resolver(), model: ctx.Model(), lang: ctx.Kind}
	// A payload value may own body-expression members, typed as the type pass does.
	bodies := &typeChecker{resolver: ctx.Resolver(), expr: expr, lang: ctx.Kind}
	expr.walkMembers = bodies.walk
	c := &sendActionChecker{
		ctx:        ctx,
		expr:       expr,
		bindings:   &w9cBindingChecker{model: ctx.Model(), resolver: ctx.Resolver(), idx: ctx.Index},
		occurrence: w8cLibraryType(ctx, "Occurrences::Occurrence"),
	}
	c.walk(rootScope, root.Members)
	return append(append(c.diags, bodies.diags...), expr.diagnostics()...)
}

type sendActionChecker struct {
	ctx        *Context
	expr       *exprChecker
	bindings   *w9cBindingChecker
	occurrence *symbols.Symbol
	diags      []Diagnostic
}

func (c *sendActionChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, unwrapMembership(member))
	}
}

// walkNode descends through every body shape a send may be written in.
func (c *sendActionChecker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Package:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Namespace:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Definition:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.SubjectMember:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.ConstraintMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.AssumeMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.RequireMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.EntryMember:
		c.walkSubactions(scope, n.Actions)
	case *ast.DoMember:
		c.walkSubactions(scope, n.Actions)
	case *ast.ExitMember:
		c.walkSubactions(scope, n.Actions)
	case *ast.InitialNode, *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walk(childScopeOr(scope, n), ast.NodeBodyMembers(n))
	case *ast.StateNode:
		body := childScopeOr(scope, n)
		c.walkSubactions(body, n.Entry)
		c.walkSubactions(body, n.Do)
		c.walkSubactions(body, n.Exit)
		c.walk(body, n.Substates)
		for _, region := range n.Regions {
			c.walkNode(body, region)
		}
	case *ast.StateRegion:
		c.walk(childScopeOr(scope, n), n.States)
	case *ast.TransitionMember:
		body := symbols.TriggerScope(scope, n)
		c.walkSubactions(body, n.Effect)
		c.walk(body, n.Members)
	case *ast.SendStatement:
		c.check(scope, n)
	case *ast.SuccessionEdge:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.WhileLoopActionNode:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			c.walkNode(scope, branch)
		}
	case *ast.IfBranchNode:
		c.walk(childScopeOr(scope, n), n.Body)
	}
}

// walkSubactions walks state subactions or transition effects; a send written as
// one, bare or as an action node's one statement, must carry a payload.
func (c *sendActionChecker) walkSubactions(scope *symbols.Scope, actions []ast.Node) {
	for _, action := range actions {
		switch n := unwrapMembership(action).(type) {
		case *ast.SendStatement:
			c.checkPayload(n)
			c.check(scope, n)
		case *ast.Usage:
			if send := subactionSend(n); send != nil {
				c.checkPayload(send)
			}
			c.walkNode(scope, n)
		default:
			c.walkNode(scope, n)
		}
	}
}

// subactionSend returns the send an action node is written as (`action s send …`);
// a send inside a braced body is the body's own member, not the subaction.
func subactionSend(usage *ast.Usage) *ast.SendStatement {
	if usage.Kind != ast.UsageAction || !usage.IsActionNode || len(usage.Members) != 1 {
		return nil
	}
	send, _ := unwrapMembership(usage.Members[0]).(*ast.SendStatement)
	return send
}

// checkPayload reports a state subaction or transition effect send that names
// no payload, in its argument or in its body.
func (c *sendActionChecker) checkPayload(send *ast.SendStatement) {
	if send.Message != nil || lower.SendPayload(send) != nil {
		return
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     send.Span(),
		Message: "a send action written as a state subaction or a transition effect must have a payload: " +
			"name the message it sends, as in `send new Msg() to receiver`",
		Code:   CodeSendPayloadMissing,
		Source: sendActionSource,
	})
}

// check types the arguments of one send — its payload, in the argument or bound
// in the body — and checks its sender and receiver; the body's other
// declarations are the type checker's.
func (c *sendActionChecker) check(scope *symbols.Scope, send *ast.SendStatement) {
	if send.Message != nil && !c.ctx.DownstreamOfFailure(send.Message) {
		c.expr.infer(scope, send.Message)
	}
	body := childScopeOr(scope, send)
	if payload := lower.SendPayloadParameter(send); payload != nil && !c.ctx.DownstreamOfFailure(payload) {
		c.expr.checkDeclValue(body, usageDecl(payload))
	}
	receiver := send.Target
	if send.IsVia {
		receiver = send.Receiver
		c.checkSender(scope, send.Target)
	}
	c.checkReceiver(scope, receiver)
	c.walk(body, send.Members)
}

// sendArgument is a send argument as classified: the feature it names, or else
// the type of the non-occurrence value it computes (scalar, result, instance).
type sendArgument struct {
	feature   *symbols.Symbol
	valueType string
}

// argument types and classifies a send argument; an untypable value and a
// non-feature referent (the feature-reference rule's to report) stay unclassified.
func (c *sendActionChecker) argument(scope *symbols.Scope, arg ast.Node) (sendArgument, bool) {
	if arg == nil || c.ctx.DownstreamOfFailure(arg) {
		return sendArgument{}, false
	}
	prim := c.expr.infer(scope, arg)
	sym, ok := c.ctx.Resolver().ResolveTarget(scope, arg)
	if ok {
		sym, ok = c.ctx.Resolver().ResolveAliasTarget(sym)
	}
	if ok && sym != nil {
		if isUsageKind(sym.Kind) {
			return sendArgument{feature: sym}, true
		}
		return sendArgument{}, false
	}
	if prim != semantics.PrimUnknown {
		return sendArgument{valueType: prim.String()}, true
	}
	typ := c.expr.constructedTypeSymbol(scope, arg)
	if typ != nil && !isTypeKind(typ.Kind) {
		return sendArgument{}, false
	}
	if typ == nil {
		typ = c.expr.invocationResultTypeSymbol(scope, arg)
	}
	model := c.bindings.model
	if typ == nil || c.occurrence == nil || model == nil ||
		w9cTypesConform(model, []*symbols.Symbol{typ}, []*symbols.Symbol{c.occurrence}) {
		return sendArgument{}, false
	}
	return sendArgument{valueType: typ.Name}, true
}

// checkReceiver checks the `to` argument, which binds SendPerformance::receiver
// (an Occurrence): a port is addressed with `via`, and a non-occurrence cannot receive.
func (c *sendActionChecker) checkReceiver(scope *symbols.Scope, receiver ast.Node) {
	arg, ok := c.argument(scope, receiver)
	if !ok {
		return
	}
	if arg.valueType != "" {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityWarning,
			Span:     receiver.Span(),
			Message: fmt.Sprintf("the receiver of a send must be an occurrence: this expression yields a %s, which is not one; "+
				"name a part, item or other occurrence after 'to'", arg.valueType),
			Code:   CodeSendReceiverNotOccurrence,
			Source: sendActionSource,
		})
		return
	}
	sym := arg.feature
	if sym.Kind == symbols.SymbolPortUsage {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityWarning,
			Span:     receiver.Span(),
			Message: fmt.Sprintf("sending to the port %s should use 'via' rather than 'to': "+
				"'via' routes the message through a port of the sender, while 'to' names the receiver", sym.Name),
			Code:   CodeSendToPort,
			Source: sendActionSource,
		})
		return
	}
	if types, ok := c.nonOccurrenceTypes(sym); ok {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityWarning,
			Span:     receiver.Span(),
			Message: fmt.Sprintf("the receiver of a send must be an occurrence: %s is typed by %s, which is not one; "+
				"name a part, item or other occurrence after 'to'", sym.Name, types),
			Code:   CodeSendReceiverNotOccurrence,
			Source: sendActionSource,
		})
	}
}

// checkSender checks the `via` argument, which binds SendPerformance::sender (an Occurrence).
func (c *sendActionChecker) checkSender(scope *symbols.Scope, sender ast.Node) {
	arg, ok := c.argument(scope, sender)
	if !ok {
		return
	}
	if arg.valueType != "" {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityWarning,
			Span:     sender.Span(),
			Message: fmt.Sprintf("a send is routed through an occurrence of the sender: this expression yields a %s, which is not one; "+
				"name a port after 'via'", arg.valueType),
			Code:   CodeSendSenderNotOccurrence,
			Source: sendActionSource,
		})
		return
	}
	if types, ok := c.nonOccurrenceTypes(arg.feature); ok {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityWarning,
			Span:     sender.Span(),
			Message: fmt.Sprintf("a send is routed through an occurrence of the sender: %s is typed by %s, which is not one; "+
				"name a port after 'via'", arg.feature.Name, types),
			Code:   CodeSendSenderNotOccurrence,
			Source: sendActionSource,
		})
	}
}

// nonOccurrenceTypes names a feature's effective types (semantics.FeatureTypes)
// when none conforms to Occurrence in either direction, as a binding requires.
func (c *sendActionChecker) nonOccurrenceTypes(sym *symbols.Symbol) (string, bool) {
	model := c.bindings.model
	if c.occurrence == nil || model == nil || model.Conforms(sym, c.occurrence) {
		return "", false
	}
	types := w8cMostSpecific(model, model.FeatureTypes(sym))
	if len(types) == 0 || w9cTypesConform(model, types, []*symbols.Symbol{c.occurrence}) {
		return "", false
	}
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name
	}
	return strings.Join(names, ", "), true
}
