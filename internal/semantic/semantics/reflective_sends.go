package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Argument positions of a send action (SysML v2.1 §8.3.17.15 SendActionUsage:
// payloadArgument = argument(1), senderArgument = argument(2), receiverArgument = argument(3)).
const (
	sendPayload = iota
	sendSender
	sendReceiver
)

// sendParts are the parameters a send action owns, in argument order, and
// their values; a parameter written empty (the sender of `send x to q`, the
// payload of `send via p`) has none, and one not written is nil.
type sendParts struct {
	params [3]*symbols.Symbol
	values [3]*symbols.Symbol
}

// sendPartsOf are the parts of the send action sym, synthesized once per model so every
// read answers the same identities; nil for any other symbol.
func (m *Model) sendPartsOf(sym *symbols.Symbol) *sendParts {
	if m == nil || sym == nil || sym.Kind != symbols.SymbolSendActionUsage || sym.Implicit != nil || sym.Expr != nil {
		return nil
	}
	send := symbols.SendStatementOf(sym.Decl)
	if send == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if parts, ok := m.sendParts[sym]; ok {
		return parts
	}
	if m.sendParts == nil {
		m.sendParts = make(map[*symbols.Symbol]*sendParts)
	}
	journal(m, m.sendParts, sym, sym.Decl)
	parts := &sendParts{}
	var arguments [3]ast.Node
	var written [3]bool
	arguments[sendPayload], written[sendPayload] = send.Message, true
	switch {
	case send.IsVia:
		arguments[sendSender], written[sendSender] = send.Target, true
		if send.Receiver != nil {
			arguments[sendReceiver], written[sendReceiver] = send.Receiver, true
		}
	case send.Target != nil:
		// A `to` with no `via` leaves the sender parameter empty before the receiver's.
		written[sendSender] = true
		arguments[sendReceiver], written[sendReceiver] = send.Target, true
	}
	for i, argument := range arguments {
		if !written[i] {
			continue
		}
		m.bindArgumentReference(sym.Scope, argument)
		param := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true, Direction: ast.DirIn, Value: argument}
		param.NodeSpan = sendParameterSpan(send, arguments, i)
		parts.params[i] = transitionPart(sym, param, param.NodeSpan, symbols.SymbolReferenceUsage, sym, symbols.ExpressionPayload)
		parts.params[i].Expr.Index = i
		if argument != nil {
			parts.values[i] = transitionPart(sym, argument, argument.Span(), symbols.SymbolExpression, parts.params[i], symbols.ExpressionValue)
		}
	}
	m.sendParts[sym] = parts
	return parts
}

// bindArgumentReference binds the name an argument expression writes, as the accept's
// receiver is bound, so its referent reads where no document walk ran.
func (m *Model) bindArgumentReference(scope *symbols.Scope, argument ast.Node) {
	slot, name, _ := expressionReference(argument)
	if name == nil || scope == nil || m.resolver == nil {
		return
	}
	if slot == "function" {
		m.resolver.ResolveInvocationName(scope, name)
		return
	}
	m.resolver.ResolveQualified(scope, name)
}

// sendParameterSpan places parameter i of send: at its argument, or, for one
// written empty, where the next written argument begins.
func sendParameterSpan(send *ast.SendStatement, arguments [3]ast.Node, i int) source.Span {
	if arguments[i] != nil {
		return arguments[i].Span()
	}
	for _, next := range arguments[i+1:] {
		if next != nil {
			return source.Span{Offset: next.Span().Offset}
		}
	}
	return source.Span{Offset: send.Span().End()}
}

// sendParameters are the parameters of the send action sym in argument order; nil for
// any other symbol.
func (m *Model) sendParameters(sym *symbols.Symbol) []*symbols.Symbol {
	parts := m.sendPartsOf(sym)
	if parts == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, param := range parts.params {
		if param != nil {
			out = append(out, param)
		}
	}
	return out
}

// sendParameterValue is the value of the send action parameter param, nil for an empty
// parameter or any other symbol.
func (m *Model) sendParameterValue(param *symbols.Symbol) *symbols.Symbol {
	if param == nil || param.Expr == nil || param.Expr.Role != symbols.ExpressionPayload {
		return nil
	}
	parts := m.sendPartsOf(param.Expr.Owner)
	if parts == nil {
		return nil
	}
	for i, candidate := range parts.params {
		if candidate == param {
			return parts.values[i]
		}
	}
	return nil
}

// nodeParameters are the parameters an accept or send action owns beyond its
// declared members, in argument order; nil for any other symbol.
func (m *Model) nodeParameters(sym *symbols.Symbol) []*symbols.Symbol {
	if params := m.acceptParameters(sym); params != nil {
		return params
	}
	return m.sendParameters(sym)
}

// sendElements is ReflectiveElements for the arguments of a send action
// (SysML v2.1 §8.3.17.15 SendActionUsage).
func (m *Model) sendElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	parts := m.sendPartsOf(sym)
	if parts == nil {
		return nil, false
	}
	switch feature {
	case "payloadArgument":
		return optionalElement(parts.values[sendPayload]), true
	case "senderArgument":
		return optionalElement(parts.values[sendSender]), true
	case "receiverArgument":
		return optionalElement(parts.values[sendReceiver]), true
	}
	return nil, false
}
