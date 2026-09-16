package migrate

import (
	"html"
	"math"
	"math/big"
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
		refs, ok := exprRefs(body)
		if !ok {
			return "", false, "opaque expression is not v2 expression syntax" + langNote(lang)
		}
		if missing := m.invisible(refs, scope); missing != "" {
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

// typingIndividual returns p's default individual when it can type the usage
// p is written as: a plain ref takes any, a part or constraint one of its kind
// that is an instance of p's type, a port none. The note says why it cannot.
func (m *migration) typingIndividual(p *xmi.Element, kw string) (*xmi.Element, string) {
	ind := m.defaultIndividual(p)
	if ind == nil || kw == "ref" {
		return ind, ""
	}
	if kw == "port" {
		return nil, "the individual " + qualifiedName(ind) + " cannot type a port: v2 has no individual port def"
	}
	kind, classifiers, _ := m.individualClassifiers(ind)
	if kind == catNone || kind.keyword() != kw+" def" {
		return nil, "the individual " + qualifiedName(ind) + " is an " + individualKeyword(kind) + ", which cannot type " + article(kw) + kw
	}
	if t := m.model.Ref(p, "type"); t != nil && !m.instanceOf(classifiers, t) {
		return nil, "the individual " + qualifiedName(ind) + " is not an instance of " + qualifiedName(t) + ", the type of " + p.Name
	}
	return ind, ""
}

// featureValue writes value v of feature f. A literal of another kind that
// spells a value of f's scalar type, as tools store a typed-in default,
// becomes that value: a string spelling a number, a whole real for an integer.
// A literal that spells no value of that type is refused, not copied.
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
	if sv == "" || !strings.HasPrefix(v.Type, "Literal") || v.Type == "LiteralNull" {
		return expr, ok, note
	}
	kind := strings.ToLower(strings.TrimPrefix(v.Type, "Literal"))
	if kind == "unlimitednatural" {
		kind = "integer"
	}
	value, spelled := scalarLiteral(kind, expr, strings.TrimSpace(v.Attrs["value"]), sv)
	switch {
	case !spelled:
		return "", false, "the " + kind + " " + expr + " is not a value of " + sv + ", which the feature holds"
	case value != expr:
		return value, true, joinNotes(note, "the "+kind+" "+expr+" is written as the "+sv+" the feature holds")
	}
	return expr, ok, note
}

// scalarLiteral rewrites a literal of the given kind — integer, real, boolean
// or string, written expr with source text — as the value of scalar type sv it
// spells; spelled is false when it spells none.
func scalarLiteral(kind, expr, text, sv string) (value string, spelled bool) {
	numeric := sv == "Real" || sv == "Rational" || sv == "Number" || sv == "Complex"
	whole := sv == "Integer" || sv == "Natural"
	switch kind {
	case "integer":
		return expr, numeric || sv == "Integer" || (sv == "Natural" && !strings.HasPrefix(expr, "-"))
	case "real":
		if numeric {
			return expr, true
		}
		if !whole {
			return "", false
		}
		// Exact, so a whole real beyond float64's integers keeps its digits.
		r, ok := new(big.Rat).SetString(expr)
		if !ok || !r.IsInt() || (sv == "Natural" && r.Sign() < 0) {
			return "", false
		}
		return r.Num().String(), true
	case "boolean":
		return expr, sv == "Boolean"
	case "string":
		switch {
		case sv == "String":
			return expr, true
		case numeric && decimal(text):
			if _, err := strconv.ParseFloat(text, 64); err != nil {
				return "", false
			}
			if !strings.ContainsAny(text, ".eE") {
				text += ".0"
			}
			return text, true
		case whole && decimal(text):
			if _, err := strconv.ParseInt(text, 10, 64); err != nil || (sv == "Natural" && text[0] == '-') {
				return "", false
			}
			return text, true
		case sv == "Boolean" && (text == "true" || text == "false"):
			return text, true
		}
		return "", false
	}
	return expr, true
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

// reference is one name an expression refers to: the segments of a qualified
// name, then the members feature chains select from it (chain is true).
type reference struct {
	global bool
	steps  []step
}

type step struct {
	name  string
	chain bool
}

// text writes the reference's first n steps as the expression spells them.
func (r reference) text(n int) string {
	var b strings.Builder
	if r.global {
		b.WriteString("$")
	}
	for i, s := range r.steps[:n] {
		switch {
		case s.chain:
			b.WriteString(".")
		case i > 0 || r.global:
			b.WriteString("::")
		}
		b.WriteString(writeName(s.name))
	}
	return b.String()
}

// exprRefs parses text as one v2 expression — the value of an attribute,
// leaving no diagnostic — and returns every name it refers to: the features,
// functions and types its meaning depends on, each with its whole path.
func exprRefs(text string) (refs []reference, ok bool) {
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
	return nameRefs(u.Value, nil), true
}

// nameRefs appends to refs each name an expression refers to. A feature chain
// whose operand is itself a name extends that name; on any other operand its
// members cannot be checked apart from it, so only the operand's names count.
func nameRefs(n ast.Node, refs []reference) []reference {
	name := func(q *ast.QualifiedName) {
		if r, ok := qualifiedRef(q); ok {
			refs = append(refs, r)
		}
	}
	switch e := n.(type) {
	case *ast.QualifiedName:
		name(e)
	case *ast.FeatureReference:
		name(e.Name)
	case *ast.FeatureChainExpr:
		if r, ok := chainRef(e); ok {
			refs = append(refs, r)
		} else {
			refs = nameRefs(e.Operand, refs)
		}
	case *ast.OperatorExpr:
		for _, o := range e.Operands {
			refs = nameRefs(o, refs)
		}
		name(e.TypeRef)
	case *ast.IndexExpr:
		refs = nameRefs(e.Operand, refs)
		refs = nameRefs(e.Index, refs)
	case *ast.InvocationExpr:
		refs = nameRefs(e.Operand, refs)
		name(e.Type)
		for _, a := range e.Args {
			refs = nameRefs(a, refs)
		}
		for _, a := range e.NamedArgs {
			refs = nameRefs(a.Value, refs)
		}
	case *ast.CollectExpr:
		refs = nameRefs(e.Operand, refs)
		refs = nameRefs(e.Body, refs)
	case *ast.SelectExpr:
		refs = nameRefs(e.Operand, refs)
		refs = nameRefs(e.Body, refs)
	case *ast.ConstructorExpr:
		name(e.Type)
		for _, a := range e.Args {
			refs = nameRefs(a, refs)
		}
		for _, a := range e.NamedArgs {
			refs = nameRefs(a.Value, refs)
		}
	case *ast.BodyExpr:
		refs = nameRefs(e.Result, refs)
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			refs = nameRefs(el, refs)
		}
	case *ast.MetadataAccessExpr:
		name(e.Ref)
	case *ast.CastExpr:
		name(e.TargetType)
	}
	return refs
}

// qualifiedRef returns the reference a qualified name spells.
func qualifiedRef(q *ast.QualifiedName) (reference, bool) {
	if q == nil || len(q.Parts) == 0 {
		return reference{}, false
	}
	r := reference{global: q.Global}
	for _, p := range q.Parts {
		r.steps = append(r.steps, step{name: p.Text})
	}
	return r, true
}

// chainRef returns the reference a feature chain spells when its operand is a
// name or another such chain, extended by the chain's member.
func chainRef(e *ast.FeatureChainExpr) (reference, bool) {
	var r reference
	var ok bool
	switch o := e.Operand.(type) {
	case *ast.FeatureReference:
		r, ok = qualifiedRef(o.Name)
	case *ast.QualifiedName:
		r, ok = qualifiedRef(o)
	case *ast.FeatureChainExpr:
		r, ok = chainRef(o)
	}
	if !ok || e.Member == nil || len(e.Member.Parts) == 0 {
		return reference{}, false
	}
	for i, p := range e.Member.Parts {
		r.steps = append(r.steps, step{name: p.Text, chain: i == 0})
	}
	return r, true
}

// invisible returns the first of the references that resolves to nothing
// written and visible from scope, as the expression spells it, or "".
func (m *migration) invisible(refs []reference, scope *xmi.Element) string {
	if len(refs) == 0 {
		return ""
	}
	visible, _ := m.visibleFrom(scope)
	for _, r := range refs {
		if _, _, missing := m.resolve(r, visible, nil); missing != "" {
			return missing
		}
	}
	return ""
}

// resolve follows a reference from the names visible in its scope through the
// members each step selects; hidden collects the private features it passes
// through, and missing spells the reference up to the step that resolves to
// nothing (when hidden is nil, a private feature is such a step).
func (m *migration) resolve(r reference, visible, hidden map[string]*xmi.Element) (e *xmi.Element, reached []*xmi.Element, missing string) {
	for i, s := range r.steps {
		var next *xmi.Element
		var private *xmi.Element
		switch {
		case i == 0 && r.global:
			next = m.rootMember(s.name)
		case i == 0:
			next, private = visible[s.name], hidden[s.name]
		default:
			next, private = m.memberNamed(e, s.name, s.chain)
		}
		if next == nil && private != nil && hidden != nil {
			next = private
			reached = append(reached, private)
		}
		if next == nil {
			return nil, nil, r.text(i + 1)
		}
		e = next
	}
	return e, reached, ""
}

// rootMember returns the written top-level element named n: a member of the
// global namespace, which the document's root Model or Package fills.
func (m *migration) rootMember(n string) *xmi.Element {
	for _, root := range m.model.Roots {
		if root.Type != "Model" {
			if m.nameOf(root) == n && m.written(root) {
				return root
			}
			continue
		}
		for _, c := range root.Children {
			if m.nameOf(c) == n && m.written(c) {
				return c
			}
		}
	}
	return nil
}

// memberNamed returns the written member of e named n, reached from outside e: a
// feature chain (chain) selects a feature of a feature's type; a qualified
// name selects any member of a namespace, or of a feature's type. private is
// the private feature the name would otherwise reach.
func (m *migration) memberNamed(e *xmi.Element, n string, chain bool) (member, private *xmi.Element) {
	visible, hidden := m.membersOf(e, chain)
	return visible[n], hidden[n]
}

// membersOf maps the names of the written members of e, seen from outside it:
// a namespace's own and inherited members, a feature's or instance's the
// members of its type or classifiers. features restricts them to features.
// Private features are hidden, as v2 neither inherits nor reaches them.
func (m *migration) membersOf(e *xmi.Element, features bool) (visible, hidden map[string]*xmi.Element) {
	visible = map[string]*xmi.Element{}
	hidden = map[string]*xmi.Element{}
	seen := map[*xmi.Element]bool{}
	var walk func(t *xmi.Element)
	walk = func(t *xmi.Element) {
		if t == nil || t.IsProxy() || seen[t] {
			return
		}
		seen[t] = true
		for _, c := range t.Children {
			n := m.nameOf(c)
			if n == "" || !m.written(c) || (features && c.Type != "Property" && c.Type != "Port") {
				continue
			}
			if m.hiddenFromHeirs(c) {
				if hidden[n] == nil {
					hidden[n] = c
				}
				continue
			}
			if visible[n] == nil {
				visible[n] = c
			}
		}
		for _, g := range t.Owned("generalization") {
			walk(m.model.Ref(g, "general"))
		}
		for _, c := range m.model.Refs(t, "classifier") {
			walk(c)
		}
	}
	switch e.Type {
	case "Property", "Port":
		walk(m.model.Ref(e, "type"))
	default:
		if features {
			// Only a feature has a feature chain.
			return visible, hidden
		}
		walk(e)
	}
	return visible, hidden
}

// visibleFrom maps each name an expression in scope can resolve to the written
// element it means: a member of scope or an enclosing namespace, a feature
// scope inherits — as a classifier from its generals, as an instance from its
// classifiers — or a member of a package one of them imports (every packaged
// element is written public). Private inherited features are not visible
// unless exposed; the hidden map holds those an expression would otherwise resolve to.
func (m *migration) visibleFrom(scope *xmi.Element) (visible, hidden map[string]*xmi.Element) {
	visible = map[string]*xmi.Element{}
	hidden = map[string]*xmi.Element{}
	seen := map[*xmi.Element]bool{}
	var members func(e *xmi.Element, inherited bool)
	members = func(e *xmi.Element, inherited bool) {
		if e == nil || seen[e] {
			return
		}
		seen[e] = true
		for _, c := range e.Children {
			n := m.nameOf(c)
			if n == "" || !m.written(c) {
				continue
			}
			if inherited && m.hiddenFromHeirs(c) {
				if hidden[n] == nil {
					hidden[n] = c
				}
				continue
			}
			if visible[n] == nil {
				visible[n] = c
			}
		}
		for _, g := range e.Owned("generalization") {
			members(m.model.Ref(g, "general"), true)
		}
		for _, c := range m.model.Refs(e, "classifier") {
			members(c, true)
		}
	}
	for cur := scope; cur != nil; cur = cur.Parent {
		members(cur, false)
		for _, p := range m.importedPackages(cur) {
			for _, c := range p.Children {
				n := m.nameOf(c)
				if n == "" || !m.written(c) || visible[n] != nil {
					continue
				}
				visible[n] = c
			}
		}
	}
	return visible, hidden
}

// importedPackages returns the packages whose members ns imports and the
// migration writes as `public import P::*`: those in the document outside a library.
func (m *migration) importedPackages(ns *xmi.Element) []*xmi.Element {
	var out []*xmi.Element
	for _, imp := range ns.Owned("packageImport") {
		if imp.Type != "PackageImport" {
			continue
		}
		if p := m.model.Ref(imp, "importedPackage"); p != nil && !p.IsProxy() && !m.isLibrary(p) {
			out = append(out, p)
		}
	}
	return out
}

// hiddenFromHeirs reports whether feature f is written private, which v2 does
// not inherit: a private or package property that nothing exposes and that is
// not a constraint parameter.
func (m *migration) hiddenFromHeirs(f *xmi.Element) bool {
	if f.Type != "Property" && f.Type != "Port" {
		return false
	}
	if vis := f.Attrs["visibility"]; vis != "private" && vis != "package" {
		return false
	}
	if m.exposed[f] != "" {
		return false
	}
	kw, _, _ := m.featureKeyword(f, m.classifyParent(f))
	return !(m.classifyParent(f) == catConstraintDef && kw == "attribute")
}

// exposeNamed marks the private features an opaque expression in scope
// reaches, inherited or through a chain, so its v2 copy can resolve them.
func (m *migration) exposeNamed(v, scope *xmi.Element) {
	body, _ := opaqueBody(v)
	refs, ok := exprRefs(body)
	if body == "" || !ok {
		return
	}
	visible, hidden := m.visibleFrom(scope)
	var reached []*xmi.Element
	for _, r := range refs {
		_, through, missing := m.resolve(r, visible, hidden)
		if missing != "" {
			return
		}
		reached = append(reached, through...)
	}
	for _, f := range reached {
		m.expose(f, "an expression in "+qualifiedName(scope)+" names it")
	}
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
