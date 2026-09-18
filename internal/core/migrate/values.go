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
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// valueExpr writes a UML value specification as a v2 expression. ok is false
// when it has no v2 form; note explains an approximation or the refusal.
func (m *migration) valueExpr(v, scope *xmi.Element) (expr string, ok bool, note string) {
	return m.valueExprAs(v, scope, wanted{})
}

// valueExprAs writes a value specification yielding what want asks for: an
// opaque body in the translated subset is translated, else copied when it is
// already v2 whose names resolve from scope.
func (m *migration) valueExprAs(v, scope *xmi.Element, want wanted) (expr string, ok bool, note string) {
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
		var refused *refusal
		if dialectOf(lang) != dialectNone {
			expr, note, refused = m.translatedExpr(body, lang, scope, want)
			if refused == nil {
				m.noted(valueOwner(v, scope), note)
				return expr, true, ""
			}
			if refused.final(lang) {
				return "", false, refused.note()
			}
		}
		refs, ok := exprRefs(body)
		if !ok {
			return "", false, refusedNote(refused, "opaque expression is not v2 expression syntax"+langNote(lang), lang)
		}
		if problem := m.invisible(refs, scope); problem != "" {
			return "", false, refusedNote(refused, "opaque expression "+problem+langNote(lang), lang)
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

// valueOwner is the element whose report entry describes value v: the element
// holding it, else the scope it is read in.
func valueOwner(v, scope *xmi.Element) *xmi.Element {
	if v.Parent != nil {
		return v.Parent
	}
	return scope
}

// featureValue writes value v of feature f. A literal of another kind that
// spells a value of f's scalar type, as tools store a typed-in default,
// becomes that value: a string spelling a number, a whole real for an integer.
// A literal that spells no value of that type is refused, not copied.
func (m *migration) featureValue(v, f, scope *xmi.Element) (expr string, ok bool, note string) {
	t := m.model.Ref(f, "type")
	expr, ok, note = m.valueExprAs(v, scope, m.wantedOf(f))
	if !ok {
		return expr, ok, note
	}
	if v.Type == "InstanceValue" && t != nil {
		inst := m.model.Ref(v, "instance")
		if inst.Type == "InstanceSpecification" && !m.instanceOf(m.model.Refs(inst, "classifier"), t) {
			return "", false, "the instance " + qualifiedName(inst) + " is not a " + qualifiedName(t) + ", which the feature holds"
		}
		if inst.Type == "EnumerationLiteral" && inst.Parent != t && m.written(t) {
			return "", false, "the literal " + qualifiedName(inst) + " is not a " + qualifiedName(t) + ", which the feature holds"
		}
	}
	sv := m.scalarBase(t)
	if sv == "" && strings.HasPrefix(v.Type, "Literal") && v.Type != "LiteralNull" && (m.structuredValueType(t) || m.written(t)) {
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
			if _, ok := new(big.Rat).SetString(text); !ok {
				return "", false
			}
			if !strings.ContainsAny(text, ".eE") {
				text += ".0"
			}
			return text, true
		case whole && decimal(text):
			if _, ok := new(big.Int).SetString(text, 10); !ok || (sv == "Natural" && text[0] == '-') {
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
// name, then the members feature chains select from it (chain is true). A
// chain on a name the expression's own body declares (local) starts from that
// local's declared type instead, whose segments are the first typed steps.
type reference struct {
	global bool
	local  string
	typed  int
	steps  []step
	// start is the offset of the first step in the expression text.
	start int
}

type step struct {
	name  string
	chain bool
}

// text writes the reference's first n steps as the expression spells them.
func (r reference) text(n int) string {
	var b strings.Builder
	steps := r.steps[:n]
	if r.local != "" && n > r.typed {
		b.WriteString(writeName(r.local))
		steps = steps[r.typed:]
	} else if r.global {
		b.WriteString("$")
	}
	for i, s := range steps {
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

// locals maps the names a body declares to the type each is declared with;
// nil for none.
type locals map[string]*ast.QualifiedName

func (l locals) has(n string) bool {
	_, ok := l[n]
	return ok
}

// exprRefs parses text as one v2 expression and returns every model name it
// refers to with its path; ok is false if it is not one, or has unread members.
func exprRefs(text string) (refs []reference, ok bool) {
	value, ok := parseExpr(text)
	if !ok {
		return nil, false
	}
	var c refCollector
	c.expr(value, nil)
	if c.unread {
		return nil, false
	}
	for i := range c.refs {
		c.refs[i].start -= len(exprProbePrefix)
	}
	return c.refs, true
}

// parseExpr parses text as one v2 expression, leaving no diagnostic.
func parseExpr(text string) (ast.Node, bool) {
	src := source.New("probe.sysml", []byte(exprProbePrefix+text+";"))
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
	if !ok || u.Value == nil || u.HasBody || len(u.Members) != 0 {
		return nil, false
	}
	return u.Value, true
}

// exprLiteral reports the kind of literal text is (integer, real, string or boolean,
// as featureValue names them) with its value text, or "" when it is not one.
func exprLiteral(text string) (kind, value string) {
	switch v, _ := parseExpr(text); lit := v.(type) {
	case *ast.LiteralInteger:
		return "integer", lit.Value
	case *ast.LiteralReal:
		return "real", lit.Value
	case *ast.LiteralString:
		return "string", strings.Trim(lit.Value, `"`)
	case *ast.LiteralBool:
		return "boolean", strconv.FormatBool(lit.Value)
	}
	return "", ""
}

// exprProbePrefix precedes an expression parsed on its own as an attribute's value.
const exprProbePrefix = "attribute probe = "

// refCollector gathers the names an expression refers to beyond its local
// ones; unread is set when a member of a kind the walk does not read is met.
type refCollector struct {
	refs   []reference
	unread bool
}

func (c *refCollector) name(q *ast.QualifiedName, local locals) {
	if r, ok := qualifiedRef(q); ok && !local.has(r.first()) {
		c.refs = append(c.refs, r)
	}
}

// chain records a feature chain; one on a body-local name is rerooted at the
// local's declared type, or left with no type steps when it declares none.
func (c *refCollector) chain(r reference, local locals) {
	if !local.has(r.first()) {
		c.refs = append(c.refs, r)
		return
	}
	chain := r.steps[1:]
	for _, s := range chain {
		if !s.chain {
			return // a qualified name under a local, which v2 cannot spell
		}
	}
	rooted := reference{local: r.steps[0].name}
	if t, ok := qualifiedRef(local[r.first()]); ok && !local.has(t.first()) {
		rooted.global, rooted.typed, rooted.steps = t.global, len(t.steps), t.steps
	}
	rooted.steps = append(rooted.steps, chain...)
	c.refs = append(c.refs, rooted)
}

// expr walks one expression; a feature chain on a name extends it, on anything
// else only the operand counts.
func (c *refCollector) expr(n ast.Node, local locals) {
	switch e := n.(type) {
	case *ast.QualifiedName:
		c.name(e, local)
	case *ast.FeatureReference:
		c.name(e.Name, local)
	case *ast.FeatureChainExpr:
		if r, ok := chainRef(e); ok {
			c.chain(r, local)
		} else {
			c.expr(e.Operand, local)
		}
	case *ast.OperatorExpr:
		for _, o := range e.Operands {
			c.expr(o, local)
		}
		c.name(e.TypeRef, local)
	case *ast.IndexExpr:
		c.expr(e.Operand, local)
		c.expr(e.Index, local)
	case *ast.InvocationExpr:
		c.expr(e.Operand, local)
		c.name(e.Type, local)
		for _, a := range e.Args {
			c.expr(a, local)
		}
		for _, a := range e.NamedArgs {
			c.expr(a.Value, local)
		}
	case *ast.CollectExpr:
		c.expr(e.Operand, local)
		c.expr(e.Body, local)
	case *ast.SelectExpr:
		c.expr(e.Operand, local)
		c.expr(e.Body, local)
	case *ast.ConstructorExpr:
		c.name(e.Type, local)
		for _, a := range e.Args {
			c.expr(a, local)
		}
		for _, a := range e.NamedArgs {
			c.expr(a.Value, local)
		}
	case *ast.BodyExpr:
		c.body(e, local)
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			c.expr(el, local)
		}
	case *ast.MetadataAccessExpr:
		c.name(e.Ref, local)
	case *ast.CastExpr:
		c.name(e.TargetType, local)
		c.multiplicity(e.Multiplicity, local)
	}
}

// body walks a body expression; its parameters and members are in scope
// throughout the body.
func (c *refCollector) body(e *ast.BodyExpr, outer locals) {
	local := scope(outer, e.Members)
	for _, p := range e.Params {
		local[p.Name] = p.Type
	}
	for _, p := range e.Params {
		c.name(p.Type, local)
		c.multiplicity(p.Multiplicity, local)
		c.relationships(p.Relationships, local)
		c.expr(p.Value, local)
		c.members(p.Members, local)
	}
	c.declarations(e.Members, local)
	c.expr(e.Result, local)
}

// members walks the declarations of a usage's or parameter's body, which are
// in scope throughout that body.
func (c *refCollector) members(members []ast.Node, outer locals) {
	if len(members) == 0 {
		return
	}
	c.declarations(members, scope(outer, members))
}

// declarations walks each member's references with its body's scope in force.
func (c *refCollector) declarations(members []ast.Node, local locals) {
	for _, m := range members {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		switch e := m.(type) {
		case *ast.Usage:
			c.usage(e, local)
		case *ast.Definition:
			c.relationships(e.Relationships, local)
			c.multiplicity(e.Multiplicity, local)
			c.members(e.Members, local)
		case *ast.ConstraintMember:
			c.expr(e.Expression, local)
			c.members(e.Body, local)
		case *ast.Comment, *ast.Documentation, *ast.TextualRepresentation:
		default:
			c.unread = true
		}
	}
}

func endScope(local locals, ends []*ast.ConnectorEnd) locals {
	var scoped locals
	for _, end := range ends {
		if end == nil {
			continue
		}
		id, declares := end.DeclaredName()
		if !declares {
			continue
		}
		if scoped == nil {
			scoped = scope(local, nil)
		}
		scoped[id.Name] = nil
	}
	if scoped == nil {
		return local
	}
	return scoped
}

func (c *refCollector) usage(e *ast.Usage, local locals) {
	c.relationships(e.Relationships, local)
	c.multiplicity(e.Multiplicity, local)
	c.expr(e.Value, local)
	if e.CrossFeature != nil {
		c.unread = true
	}
	for _, end := range e.ConnectorEnds {
		if end == nil {
			continue
		}
		redefines, others := ast.SplitRedefinitions(end.Relationships)
		if len(redefines) != 0 {
			c.unread = true
		}
		c.relationships(others, local)
		c.multiplicity(end.Multiplicity, local)
		if _, declares := end.DeclaredName(); !declares {
			c.expr(end.Target, local)
		}
		c.expr(end.Reference, local)
	}
	if e.FlowEnds != nil {
		c.expr(e.FlowEnds.From, local)
		c.expr(e.FlowEnds.To, local)
		if e.FlowEnds.PayloadDecl != nil {
			c.unread = true
		} else {
			c.expr(e.FlowEnds.Payload, local)
		}
		c.multiplicity(e.FlowEnds.PayloadMultiplicity, local)
	}
	c.members(e.Members, endScope(local, e.ConnectorEnds))
}

func (c *refCollector) relationships(rels []*ast.Relationship, local locals) {
	for _, rel := range rels {
		c.expr(rel.Target, local)
		c.multiplicity(rel.Multiplicity, local)
	}
}

func (c *refCollector) multiplicity(m *ast.Multiplicity, local locals) {
	if m == nil {
		return
	}
	c.expr(m.Lower, local)
	c.expr(m.Upper, local)
}

// scope returns outer extended by the names members declare, each with the
// one type a usage is declared with.
func scope(outer locals, members []ast.Node) locals {
	local := make(locals, len(outer)+len(members))
	for n, t := range outer {
		local[n] = t
	}
	for _, m := range members {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		var id ast.Identification
		var t *ast.QualifiedName
		switch e := m.(type) {
		case *ast.Usage:
			id, t = e.Ident, soleType(e.Relationships)
		case *ast.Definition:
			id = e.Ident
		case *ast.ConstraintMember:
			id.Name = e.Name
		}
		if id.Name != "" {
			local[id.Name] = t
		}
		if id.ShortName != "" {
			local[id.ShortName] = t
		}
	}
	return local
}

// soleType returns the one type a usage's relationships name, or nil when it
// names none, several, or one by an expression.
func soleType(rels []*ast.Relationship) *ast.QualifiedName {
	var t *ast.QualifiedName
	for _, rel := range rels {
		if rel.Kind != ast.RelTyping {
			continue
		}
		q, ok := rel.Target.(*ast.QualifiedName)
		if !ok || t != nil {
			return nil
		}
		t = q
	}
	return t
}

// first is the name a reference starts from; "" for a global one.
func (r reference) first() string {
	if r.global || len(r.steps) == 0 {
		return ""
	}
	return r.steps[0].name
}

// qualifiedRef returns the reference a qualified name spells.
func qualifiedRef(q *ast.QualifiedName) (reference, bool) {
	if q == nil || len(q.Parts) == 0 {
		return reference{}, false
	}
	r := reference{global: q.Global, start: q.Parts[0].Span.Offset}
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

// invisible says why the references cannot all be seen from scope: the first that
// resolves to nothing, or reaches through an untyped local. "" if all resolve.
func (m *migration) invisible(refs []reference, scope *xmi.Element) string {
	if len(refs) == 0 {
		return ""
	}
	visible, _ := m.visibleFrom(scope)
	for _, r := range refs {
		e, _, missing := m.resolve(r, visible, nil)
		if missing != "" {
			if r.local != "" && r.typed == 0 {
				return "reaches " + missing + " through " + writeName(r.local) + ", whose type it does not declare"
			}
			return "names " + missing + ", which nothing visible from " + qualifiedName(scope) + " is called"
		}
		if kw := m.notAValue(e); kw != "" {
			return "names " + r.text(len(r.steps)) + ", which is " + kw + " " + qualifiedName(e) + ", not a value an expression can read"
		}
	}
	return ""
}

// notAValue names the kind of declaration e becomes when an expression cannot
// read it: an operation or behavior written as an action or state def.
func (m *migration) notAValue(e *xmi.Element) string {
	if e == nil || (e.Type != "Operation" && !isBehavior(e)) {
		return ""
	}
	switch cat, _ := m.classify(e); cat {
	case catActionDef:
		return "the action def"
	case catStateDef:
		return "the state def"
	}
	return ""
}

// resolve follows a reference from the names visible in its scope through the
// members each step selects; hidden collects the private features it passes
// through, and missing spells the reference up to the step that resolves to
// nothing (when hidden is nil, a private feature is such a step). A name no
// written element answers is looked up in the standard library; e is nil then.
func (m *migration) resolve(r reference, visible, hidden map[string]*xmi.Element) (e *xmi.Element, reached []*xmi.Element, missing string) {
	if r.local != "" && r.typed == 0 {
		return nil, nil, r.text(len(r.steps))
	}
	var lib string
	for i, s := range r.steps {
		var next *xmi.Element
		var private *xmi.Element
		switch {
		case lib != "":
			if lib = m.libraryMember(lib, s.name, s.chain); lib == "" {
				return nil, nil, r.text(i + 1)
			}
			continue
		case i == 0 && r.global:
			next = m.rootMember(s.name)
		case i == 0:
			next, private = visible[s.name], hidden[s.name]
		case r.local != "" && i == r.typed:
			next, private = m.memberNamed(e, s.name, memberFeature)
		default:
			next, private = m.memberNamed(e, s.name, chainKind(s.chain))
		}
		if next == nil && private != nil && hidden != nil {
			next = private
			reached = append(reached, private)
		}
		if next == nil && i == 0 && !s.chain && m.libraryPackage(s.name) {
			lib = s.name
			continue
		}
		if next == nil {
			return nil, nil, r.text(i + 1)
		}
		e = next
	}
	return e, reached, ""
}

// libraryPackage reports whether n names a top-level package of the standard
// library, which a v2 model reaches without importing it.
func (m *migration) libraryPackage(n string) bool {
	for _, sym := range libs.SharedBase().LookupQualified(n) {
		if _, ok := sym.Decl.(*ast.Package); ok {
			return true
		}
	}
	return false
}

// libraryMember returns the qualified name of the library member of fqn named n,
// or ""; a feature chain also selects members inherited from fqn's supertypes.
func (m *migration) libraryMember(fqn, n string, chain bool) string {
	idx := libs.SharedBase()
	if !chain {
		if len(idx.LookupDirectChildrenNamed(fqn, n)) != 0 {
			return fqn + "::" + n
		}
		return ""
	}
	seen := map[string]bool{}
	queue := []string{fqn}
	for len(queue) != 0 {
		t := queue[0]
		queue = queue[1:]
		if seen[t] {
			continue
		}
		seen[t] = true
		if len(idx.LookupDirectChildrenNamed(t, n)) != 0 {
			return t + "::" + n
		}
		queue = append(queue, librarySupers(idx, t)...)
	}
	return ""
}

// librarySupers returns the qualified names of the direct supertypes the
// library records for fqn.
func librarySupers(idx *symbols.Index, fqn string) []string {
	var out []string
	for _, sym := range idx.LookupQualified(fqn) {
		if sym.Facts != nil {
			out = append(out, sym.Facts.Supers...)
		}
	}
	return out
}

// memberKind says which members of an element a step of a reference selects.
type memberKind int

const (
	memberAny     memberKind = iota // a qualified name: any member of a namespace or a feature's type
	memberChained                   // a feature chain: a feature of a feature's type
	memberFeature                   // a feature chain on a typed local: a feature of the type itself
)

func chainKind(chain bool) memberKind {
	if chain {
		return memberChained
	}
	return memberAny
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

// memberNamed returns the written member of e named n that a step of the given
// kind selects, reached from outside e. private is the private feature the
// name would otherwise reach.
func (m *migration) memberNamed(e *xmi.Element, n string, kind memberKind) (member, private *xmi.Element) {
	visible, hidden := m.membersOf(e, kind)
	return visible[n], hidden[n]
}

// membersOf maps the names of the written members of e, seen from outside it:
// a namespace's own and inherited members, a feature's (a parameter's too) or
// instance's the members of its type or classifiers, restricted as kind says. Private
// features are hidden, as v2 neither inherits nor reaches them.
func (m *migration) membersOf(e *xmi.Element, kind memberKind) (visible, hidden map[string]*xmi.Element) {
	visible = map[string]*xmi.Element{}
	hidden = map[string]*xmi.Element{}
	seen := map[*xmi.Element]bool{}
	features := kind != memberAny
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
	switch {
	case e.Type == "Property" || e.Type == "Port" || e.Type == "Parameter":
		if kind != memberFeature {
			walk(m.model.Ref(e, "type"))
		}
	case kind == memberChained:
		// Only a feature has a feature chain.
	default:
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
