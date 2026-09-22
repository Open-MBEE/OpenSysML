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

// treeBinary maps the symbols a tree spells a binary operator with — the sign,
// or the operator's name in any case (Plus, Equal) — to the script operator; a
// symbol over several operands folds them left.
var treeBinary = map[string]string{
	"+": "+", "-": "-", "*": "*", "/": "/", "%": "%", "mod": "%", "**": "**",
	"plus": "+", "minus": "-", "times": "*", "multiply": "*", "divide": "/", "modulo": "%",
	"<": "<", "<=": "<=", ">": ">", ">=": ">=", "≤": "<=", "≥": ">=",
	"less": "<", "lessorequal": "<=", "greater": ">", "greaterorequal": ">=",
	"==": "==", "=": "==", "!=": "!=", "<>": "!=", "≠": "!=",
	"equal": "==", "equals": "==", "notequal": "!=",
	"&&": "&&", "and": "&&", "∧": "&&", "||": "||", "or": "||", "∨": "||",
}

// treeUnary maps the symbols of a unary operator, applied to one operand.
var treeUnary = map[string]string{"-": "-", "minus": "-", "negate": "-", "!": "!", "not": "!", "¬": "!"}

// treeOperator finds symbol in an operator table as spelled, else by its name in any case.
func treeOperator(table map[string]string, symbol string) (string, bool) {
	if op, ok := table[symbol]; ok {
		return op, true
	}
	op, ok := table[strings.ToLower(symbol)]
	return op, ok && identifier(symbol)
}

// treeLowering lowers one tree read at scope; operands script text cannot spell
// — an instance value, an opaque body in its own language — are stood for by
// placeholder names it answers itself.
type treeLowering struct {
	s      *bodyScope
	leaves map[string]opaqueRef
	// spelled holds every word the tree's own symbols and opaque bodies spell,
	// which no placeholder may shadow.
	spelled map[string]bool
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
	l := &treeLowering{s: m.bodyScope(scope), leaves: map[string]opaqueRef{}, spelled: treeWords(v)}
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
	case "ElementValue":
		return l.element(v)
	case "OpaqueExpression":
		body, lang := opaqueBody(v)
		if body == "" {
			return "", &refusal{kind: refusedSyntax, token: "", why: "an opaque operand has no body"}
		}
		if !dialectOf(lang).script() {
			return "", &refusal{kind: refusedLanguage, token: lang, why: "an opaque operand is read only in a script language"}
		}
		return l.opaque(body, lang)
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
	if op, ok := treeOperator(treeUnary, symbol); ok && len(operands) == 1 {
		x, err := l.operand(operands[0])
		if err != nil {
			return "", err
		}
		return op + x, nil
	}
	if op, ok := treeOperator(treeBinary, symbol); ok && len(operands) >= 2 {
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
// dotted name as the table spells it, or a bare name of the script's Math object
// in any case (Power, MAX), with power as the tools' spelling of pow.
func treeFunction(symbol string) (string, bool) {
	if !identifierPath(symbol) {
		return "", false
	}
	if !strings.Contains(symbol, ".") {
		name := strings.ToLower(symbol)
		if name == "power" {
			name = "pow"
		}
		if pureCalls["Math."+name] {
			return "Math." + name, true
		}
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

// opaque translates an opaque operand in its own language and writes it through
// a placeholder answering the translation, so a Java body keeps Java's reading.
func (l *treeLowering) opaque(body, lang string) (string, *refusal) {
	t, err := translateExpr(body, lang, l, wanted{})
	if err != nil {
		return "", err
	}
	ref := opaqueRef{expr: t.expr, scalar: t.scalar, object: t.object, plural: t.plural, loose: t.loose, lit: t.lit}
	if !t.atomic && t.loose == 0 {
		ref.expr = "(" + t.expr + ")"
	}
	name := l.placeholder("operand")
	l.leaves[name] = ref
	return name, nil
}

// element writes an element value operand — a tool's reference to a feature —
// through a placeholder that answers what the feature's name reads from the
// scope, refusing a feature that name does not reach from there.
func (l *treeLowering) element(v *sysmlv1.Element) (string, *refusal) {
	m := l.s.m
	f := m.model.Ref(v, "element")
	if f == nil {
		why := "the element value refers to nothing in the document"
		if len(v.RefIDs("element")) == 0 {
			why = "the element value names no element"
		}
		return "", &refusal{kind: refusedConstruct, token: describeValue(v), why: why}
	}
	name := m.nameOf(f)
	if name == "" {
		return "", &refusal{kind: refusedName, token: describeValue(v), why: "the element value names an unnamed " + kindOf(f)}
	}
	a := l.s.featureAnchor([]string{name}, false)
	if a.refusal != nil {
		return "", a.refusal
	}
	if a.f != f {
		return "", &refusal{kind: refusedName, token: name,
			why: "the element value names " + kindOf(f) + " " + qualifiedName(f) + ", which is not what the name reads from " + qualifiedName(l.s.scope)}
	}
	ref, err := l.s.feature([]string{name}, false)
	if err != nil {
		return "", err
	}
	placeholder := l.placeholder(name)
	l.leaves[placeholder] = ref
	return placeholder, nil
}

// treeWords collects the words the symbols and opaque bodies under v spell.
func treeWords(v *sysmlv1.Element) map[string]bool {
	words := map[string]bool{}
	spell := func(text string) {
		for _, w := range strings.FieldsFunc(text, func(r rune) bool {
			return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$')
		}) {
			words[w] = true
		}
	}
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		spell(e.Attrs["symbol"])
		if e.Type == "OpaqueExpression" {
			body, _ := opaqueBody(e)
			spell(body)
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(v)
	return words
}

// placeholder derives from name an identifier that no word of the tree and no
// earlier placeholder spells, for an instance or element operand.
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
		if _, taken := l.leaves[candidate]; !taken && !l.spelled[candidate] {
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
