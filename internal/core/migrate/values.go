package migrate

import (
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// valueExpr writes a UML value specification as a v2 expression. ok is false
// when it has no v2 form; note explains an approximation or the refusal.
func (m *migration) valueExpr(v, scope *xmi.Element) (expr string, ok bool, note string) {
	switch v.Type {
	case "LiteralInteger", "LiteralUnlimitedNatural":
		val := v.Attrs["value"]
		if val == "" {
			val = "0"
		}
		if val == "*" {
			return "", false, "an unlimited natural value has no v2 expression outside a multiplicity"
		}
		return val, true, ""
	case "LiteralReal":
		val := v.Attrs["value"]
		if val == "" {
			val = "0.0"
		}
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", false, "real literal " + strconv.Quote(val) + " is not a finite number"
		}
		if !strings.ContainsAny(val, ".eE") {
			val += ".0"
		}
		return val, true, ""
	case "LiteralBoolean":
		switch v.Attrs["value"] {
		case "true", "1":
			return "true", true, ""
		case "false", "0", "":
			return "false", true, ""
		}
		return "", false, "boolean literal " + strconv.Quote(v.Attrs["value"]) + " is not a boolean"
	case "LiteralString":
		return lexer.StringText(v.Attrs["value"]), true, ""
	case "LiteralNull":
		return "null", true, ""
	case "InstanceValue":
		inst := m.model.Ref(v, "instance")
		if inst == nil {
			return "", false, "instance value refers to nothing in the document"
		}
		if inst.Type == "EnumerationLiteral" && inst.Parent != nil {
			return m.ref(inst.Parent, scope) + "::" + writeName(inst.Name), true, ""
		}
		switch cat, _ := m.classify(inst); cat {
		case catValue:
			return m.ref(inst, scope), true, ""
		case catIndividualDef:
			return "", false, "the individual " + qualifiedName(inst) + " is a definition, which is not a v2 value"
		}
		return "", false, "instance value of a " + inst.Type + " has no v2 expression"
	case "OpaqueExpression":
		body, lang := opaqueBody(v)
		if body == "" {
			return "", false, "opaque expression has no body"
		}
		roots, ok := exprRoots(body)
		if !ok {
			return "", false, "opaque expression is not v2 expression syntax" + langNote(lang)
		}
		if missing := m.invisible(roots, scope); missing != "" {
			return "", false, "opaque expression names " + missing + ", which nothing visible from " + qualifiedName(scope) + " is called" + langNote(lang)
		}
		return body, true, "opaque expression copied verbatim" + langNote(lang)
	case "Expression", "TimeExpression", "Duration", "Interval", "StringExpression":
		return "", false, "a UML " + v.Type + " tree has no v2 form"
	}
	return "", false, "no v2 form for a UML " + v.Type
}

func langNote(lang string) string {
	if lang == "" {
		return ""
	}
	return " (language " + lang + ")"
}

// defaultIndividual returns the individual an instance-value default of p
// names, or nil when its default is anything else.
func (m *migration) defaultIndividual(p *xmi.Element) *xmi.Element {
	dv := firstOwned(p, "defaultValue")
	if dv == nil || dv.Type != "InstanceValue" {
		return nil
	}
	inst := m.model.Ref(dv, "instance")
	if inst == nil || inst.IsProxy() {
		return nil
	}
	if cat, _ := m.classify(inst); cat != catIndividualDef {
		return nil
	}
	return inst
}

// featureValue writes value v of feature f. A literal of another kind that
// spells a value of f's scalar type, as tools store a typed-in default,
// becomes that value: a string spelling a number, a whole real for an integer.
func (m *migration) featureValue(v, f, scope *xmi.Element) (expr string, ok bool, note string) {
	expr, ok, note = m.valueExpr(v, scope)
	if !ok {
		return expr, ok, note
	}
	t := m.model.Ref(f, "type")
	sv := m.scalarBase(t)
	if sv == "" && strings.HasPrefix(v.Type, "Literal") && m.structuredValueType(t) {
		return "", false, "the literal " + expr + " is not a value of " + qualifiedName(t) + ", which has no scalar base"
	}
	if v.Type == "LiteralReal" && (sv == "Integer" || sv == "Natural") {
		r, err := strconv.ParseFloat(expr, 64)
		if err != nil || r != math.Trunc(r) || (sv == "Natural" && r < 0) {
			return expr, ok, note
		}
		return strconv.FormatFloat(r, 'f', 0, 64), true, joinNotes(note, "the real "+expr+" is written as the "+sv+" the feature holds")
	}
	if v.Type != "LiteralString" {
		return expr, ok, note
	}
	text := strings.TrimSpace(v.Attrs["value"])
	switch sv {
	case "Real", "Rational", "Number":
		if !decimal(text) {
			return expr, ok, note
		}
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			return expr, ok, note
		}
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
	case "Integer", "Natural":
		if !decimal(text) {
			return expr, ok, note
		}
		if _, err := strconv.ParseInt(text, 10, 64); err != nil || (sv == "Natural" && text[0] == '-') {
			return expr, ok, note
		}
	case "Boolean":
		if text != "true" && text != "false" {
			return expr, ok, note
		}
	default:
		return expr, ok, note
	}
	return text, true, joinNotes(note, "the string "+expr+" is written as the "+sv+" the feature holds")
}

// decimal reports whether text is spelled as a decimal number, with an
// optional sign, fraction and exponent.
func decimal(text string) bool {
	return text != "" && strings.Trim(text, "0123456789.+-eE") == "" && strings.ContainsAny(text, "0123456789")
}

// opaqueBody returns the first body of an opaque expression and its language.
func opaqueBody(v *xmi.Element) (body, lang string) {
	if b := v.Owned("body"); len(b) > 0 {
		body = strings.TrimSpace(b[0].Text)
	} else {
		body = strings.TrimSpace(v.Attrs["body"])
	}
	if l := v.Owned("language"); len(l) > 0 {
		lang = strings.TrimSpace(l[0].Text)
	} else {
		lang = strings.TrimSpace(v.Attrs["language"])
	}
	return body, lang
}

// exprRoots parses text as one v2 expression — the value of an attribute,
// leaving no diagnostic — and returns the first segment of every name it
// refers to: the features, functions and types its meaning depends on.
func exprRoots(text string) (roots []string, ok bool) {
	if strings.ContainsAny(text, ";{}") {
		return nil, false
	}
	src := source.New("probe.sysml", []byte("attribute probe = "+text+";"))
	p := parser.New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 || len(root.Members) != 1 {
		return nil, false
	}
	mem, ok := root.Members[0].(*ast.Membership)
	if !ok {
		return nil, false
	}
	u, ok := mem.Member.(*ast.Usage)
	if !ok {
		return nil, false
	}
	return nameRoots(u.Value, nil), true
}

// nameRoots appends to roots the first segment of each name an expression
// refers to.
func nameRoots(n ast.Node, roots []string) []string {
	name := func(q *ast.QualifiedName) {
		if q != nil && len(q.Parts) > 0 && !q.Global {
			roots = append(roots, q.Parts[0].Text)
		}
	}
	switch e := n.(type) {
	case *ast.QualifiedName:
		name(e)
	case *ast.FeatureReference:
		name(e.Name)
	case *ast.FeatureChainExpr:
		roots = nameRoots(e.Operand, roots)
	case *ast.OperatorExpr:
		for _, o := range e.Operands {
			roots = nameRoots(o, roots)
		}
		name(e.TypeRef)
	case *ast.IndexExpr:
		roots = nameRoots(e.Operand, roots)
		roots = nameRoots(e.Index, roots)
	case *ast.InvocationExpr:
		roots = nameRoots(e.Operand, roots)
		name(e.Type)
		for _, a := range e.Args {
			roots = nameRoots(a, roots)
		}
		for _, a := range e.NamedArgs {
			roots = nameRoots(a.Value, roots)
		}
	case *ast.CollectExpr:
		roots = nameRoots(e.Operand, roots)
		roots = nameRoots(e.Body, roots)
	case *ast.SelectExpr:
		roots = nameRoots(e.Operand, roots)
		roots = nameRoots(e.Body, roots)
	case *ast.ConstructorExpr:
		name(e.Type)
		for _, a := range e.Args {
			roots = nameRoots(a, roots)
		}
		for _, a := range e.NamedArgs {
			roots = nameRoots(a.Value, roots)
		}
	case *ast.BodyExpr:
		roots = nameRoots(e.Result, roots)
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			roots = nameRoots(el, roots)
		}
	case *ast.MetadataAccessExpr:
		name(e.Ref)
	case *ast.CastExpr:
		name(e.TargetType)
	}
	return roots
}

// invisible returns the first of the names that no element visible from
// scope answers to — as a member of scope or an enclosing namespace, a feature
// scope inherits, or a feature of an instance's classifiers — or "".
func (m *migration) invisible(names []string, scope *xmi.Element) string {
	if len(names) == 0 {
		return ""
	}
	visible := map[string]bool{}
	seen := map[*xmi.Element]bool{}
	var members func(*xmi.Element)
	members = func(e *xmi.Element) {
		if e == nil || seen[e] {
			return
		}
		seen[e] = true
		for _, c := range e.Children {
			if n := m.nameOf(c); n != "" {
				visible[n] = true
			}
		}
		for _, g := range e.Owned("generalization") {
			members(m.model.Ref(g, "general"))
		}
		for _, c := range m.model.Refs(e, "classifier") {
			members(c)
		}
	}
	for cur := scope; cur != nil; cur = cur.Parent {
		members(cur)
	}
	for _, n := range names {
		if !visible[n] {
			return writeName(n)
		}
	}
	return ""
}

var (
	htmlTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlBreak  = regexp.MustCompile(`(?i)</p>|<br\s*/?>`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

// commentText prepares a v1 comment body for a v2 comment: some tools store
// documentation as HTML, whose tags are dropped and entities decoded.
func commentText(body string) string {
	text := body
	if strings.Contains(strings.ToLower(text), "<html") || strings.Contains(text, "<p>") || strings.Contains(text, "<br") {
		text = htmlBreak.ReplaceAllString(text, "\n")
		text = htmlTag.ReplaceAllString(text, "")
		text = html.UnescapeString(text)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = blankLines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// commentLines writes a v2 comment body over one or more lines, closing any
// comment terminator the text itself contains.
func commentLines(text string) []string {
	text = strings.ReplaceAll(text, "*/", "* /")
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		return []string{"/* " + lines[0] + " */"}
	}
	out := make([]string, 0, len(lines)+1)
	for i, l := range lines {
		l = strings.TrimRight(l, " \t")
		switch {
		case i == 0:
			out = append(out, "/* "+l)
		default:
			out = append(out, " * "+l)
		}
	}
	out = append(out, " */")
	return out
}
