package migrate

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// This file lowers a UML Expression tree — a symbol over operand value
// specifications — into script text the opaque translator reads, so a tree and
// an opaque body share one operator subset, function table and scope rules.

// treeBinary maps the symbols a tree spells a binary operator with to the
// script operator; a symbol over several operands folds them left.
var treeBinary = map[string]string{
	"+": "+", "-": "-", "*": "*", "/": "/", "%": "%", "mod": "%", "**": "**",
	"<": "<", "<=": "<=", ">": ">", ">=": ">=", "≤": "<=", "≥": ">=",
	"==": "==", "=": "==", "!=": "!=", "<>": "!=", "≠": "!=",
	"&&": "&&", "and": "&&", "∧": "&&", "||": "||", "or": "||", "∨": "||",
}

// treeUnary maps the symbols of a unary operator, applied to one operand.
var treeUnary = map[string]string{"-": "-", "!": "!", "not": "!", "¬": "!"}

// treeLowering lowers one tree read at scope; operands script text cannot spell
// — an instance value — are stood for by placeholder names it answers itself.
type treeLowering struct {
	s      *bodyScope
	leaves map[string]opaqueRef
}

// feature answers a placeholder for an instance operand, else asks the scope.
func (l *treeLowering) feature(path []string, write bool) (opaqueRef, *refusal) {
	if ref, ok := l.leaves[path[0]]; ok && len(path) == 1 {
		return ref, nil
	}
	return l.s.feature(path, write)
}

// expressionTree writes a UML Expression tree as a v2 expression yielding what
// want asks for. ok is false when a node has no v2 form; note says why.
func (m *migration) expressionTree(v, scope *sysmlv1.Element, want wanted) (expr string, ok bool, note string) {
	l := &treeLowering{s: m.bodyScope(scope), leaves: map[string]opaqueRef{}}
	text, err := l.lower(v)
	if err == nil {
		expr, err = m.translateIn(text, "", l, want)
	}
	if err != nil {
		return "", false, "the UML " + v.Type + " tree has no v2 form: " + err.note()
	}
	return expr, true, joinNotes("the UML Expression tree is written as a v2 expression", l.s.note(""))
}

// lower writes value specification v as script text, refusing what the
// translated subset leaves out.
func (l *treeLowering) lower(v *sysmlv1.Element) (string, *refusal) {
	m := l.s.m
	switch v.Type {
	case "Expression", "StringExpression":
		return l.node(v)
	case "LiteralInteger", "LiteralUnlimitedNatural", "LiteralReal", "LiteralBoolean", "LiteralString":
		text, ok, note := m.directValue(v, l.s.scope, wanted{})
		if !ok {
			return "", &refusal{kind: refusedConstruct, token: describeValue(v), why: note}
		}
		return text, nil
	case "InstanceValue":
		return l.instance(v)
	case "OpaqueExpression":
		body, lang := opaqueBody(v)
		if body == "" {
			return "", &refusal{kind: refusedSyntax, token: "", why: "an opaque operand has no body"}
		}
		if dialectOf(lang) != dialectScript {
			return "", &refusal{kind: refusedLanguage, token: lang, why: "an opaque operand is read only in a script language"}
		}
		return "(" + body + ")", nil
	}
	return "", &refusal{kind: refusedConstruct, token: "<" + v.Type + ">", why: "a UML " + v.Type + " has no v2 expression"}
}

// node writes an Expression node: its operands under its symbol, a bare symbol
// as an operand, a symbol-less node as a grouping of its one operand.
func (l *treeLowering) node(v *sysmlv1.Element) (string, *refusal) {
	symbol := strings.TrimSpace(v.Attrs["symbol"])
	operands := v.Owned("operand")
	if v.Type == "StringExpression" {
		if subs := v.Owned("subExpression"); len(subs) > 0 {
			return l.fold("+", subs)
		}
	}
	switch {
	case symbol == "" && len(operands) == 1:
		return l.operand(operands[0])
	case symbol == "":
		return "", &refusal{kind: refusedConstruct, token: "<" + v.Type + ">",
			why: "the node has no symbol and " + strconv.Itoa(len(operands)) + " operands, so no operator applies"}
	case len(operands) == 0:
		return symbol, nil
	}
	if op, ok := treeUnary[symbol]; ok && len(operands) == 1 {
		x, err := l.operand(operands[0])
		if err != nil {
			return "", err
		}
		return op + x, nil
	}
	if op, ok := treeBinary[symbol]; ok && len(operands) >= 2 {
		return l.fold(op, operands)
	}
	if fn, ok := treeFunction(symbol); ok {
		args := make([]string, len(operands))
		for i, o := range operands {
			x, err := l.lower(o)
			if err != nil {
				return "", err
			}
			args[i] = x
		}
		return fn + "(" + strings.Join(args, ", ") + ")", nil
	}
	return "", &refusal{kind: refusedConstruct, token: symbol,
		why: "no operator of that symbol over " + strconv.Itoa(len(operands)) + " operand(s) is in the translated subset"}
}

// fold writes the operands joined by op, left to right.
func (l *treeLowering) fold(op string, operands []*sysmlv1.Element) (string, *refusal) {
	parts := make([]string, len(operands))
	for i, o := range operands {
		x, err := l.operand(o)
		if err != nil {
			return "", err
		}
		parts[i] = x
	}
	return strings.Join(parts, " "+op+" "), nil
}

// operand writes v as the operand of an operator: a nested node in parentheses.
func (l *treeLowering) operand(v *sysmlv1.Element) (string, *refusal) {
	x, err := l.lower(v)
	if err != nil {
		return "", err
	}
	if (v.Type == "Expression" || v.Type == "StringExpression") && strings.TrimSpace(v.Attrs["symbol"]) != "" && len(v.Owned("operand")) > 0 {
		return "(" + x + ")", nil
	}
	return x, nil
}

// treeFunction is the function of the translated table a call symbol names: a
// dotted name as the table spells it, or a bare name of the script's Math object.
func treeFunction(symbol string) (string, bool) {
	if !identifierPath(symbol) {
		return "", false
	}
	if !strings.Contains(symbol, ".") && pureCalls["Math."+symbol] {
		return "Math." + symbol, true
	}
	return symbol, true
}

// identifierPath reports whether s is one identifier or a dotted chain of them.
func identifierPath(s string) bool {
	for _, part := range strings.Split(s, ".") {
		if !identifier(part) {
			return false
		}
	}
	return true
}

// identifier reports whether s is spelled as a script identifier.
func identifier(s string) bool {
	for i, r := range s {
		if !(unicode.IsLetter(r) || r == '_' || r == '$' || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return s != ""
}

// instance writes an instance value operand through a placeholder name that
// answers the instance's v2 reference and type.
func (l *treeLowering) instance(v *sysmlv1.Element) (string, *refusal) {
	m := l.s.m
	expr, ok, note := m.instanceValue(v, l.s.scope)
	inst := m.model.Ref(v, "instance")
	if !ok {
		kind := refusedName
		if inst == nil {
			kind = refusedConstruct
		}
		return "", &refusal{kind: kind, token: describeValue(v), why: note}
	}
	typ := inst.Parent
	if inst.Type != "EnumerationLiteral" {
		typ = nil
		if cs := m.model.Refs(inst, "classifier"); len(cs) > 0 {
			typ = cs[0]
		}
	}
	name := l.placeholder(m.nameOf(inst))
	l.leaves[name] = opaqueRef{expr: expr, scalar: m.scalarBase(typ), object: m.nonScalar(typ)}
	return name, nil
}

// placeholder derives an unused identifier from name for an instance operand.
func (l *treeLowering) placeholder(name string) string {
	base := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return '_'
	}, name)
	if !identifier(base) || scriptReserved[base] || strings.EqualFold(base, "true") || strings.EqualFold(base, "false") || base == "this" {
		base = "_" + base
	}
	candidate := base
	for i := 2; ; i++ {
		if _, taken := l.leaves[candidate]; !taken {
			return candidate
		}
		candidate = base + strconv.Itoa(i)
	}
}

// treeText shows an Expression tree for a comment: the symbol over its
// operands, `symbol(a, b)`, a bare symbol as itself.
func treeText(v *sysmlv1.Element) string {
	symbol := strings.TrimSpace(v.Attrs["symbol"])
	operands := v.Owned("operand")
	if v.Type == "StringExpression" {
		operands = append(operands, v.Owned("subExpression")...)
	}
	if len(operands) == 0 {
		if symbol == "" {
			return "<" + v.Type + ">"
		}
		return symbol
	}
	parts := make([]string, len(operands))
	for i, o := range operands {
		parts[i] = describeValue(o)
	}
	return symbol + "(" + strings.Join(parts, ", ") + ")"
}
