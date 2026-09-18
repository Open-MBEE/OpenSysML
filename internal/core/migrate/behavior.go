package migrate

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// classifyBehavior decides the v2 declaration a UML behavior becomes: action def,
// state def, calc def for an expression body, or a scenario action def for an interaction.
func (m *migration) classifyBehavior(e *xmi.Element) (category, string) {
	switch e.Type {
	case "Activity":
		return catActionDef, ""
	case "StateMachine":
		return catStateDef, ""
	case "OpaqueBehavior", "FunctionBehavior":
		if m.deciding[e] {
			return catActionDef, ""
		}
		m.deciding[e] = true
		_, ok, _ := m.calcExpr(e)
		delete(m.deciding, e)
		if ok {
			return catCalcDef, ""
		}
		return catActionDef, ""
	case "Interaction":
		if note := m.interactionNote(e); note != "" {
			return catUnmapped, note
		}
		return catActionDef, ""
	}
	return catUnmapped, "no v2 form for a UML " + e.Type
}

// behaviorCategory reports whether cat is written by the behavior writers.
func behaviorCategory(cat category) bool {
	return cat == catActionDef || cat == catCalcDef || cat == catStateDef
}

// isBehavior reports whether e is a UML behavior.
func isBehavior(e *xmi.Element) bool {
	switch e.Type {
	case "Activity", "StateMachine", "OpaqueBehavior", "FunctionBehavior", "Interaction":
		return true
	}
	return false
}

// behaviorBody writes the body of a behavior or operation declaration.
func (m *migration) behaviorBody(e *xmi.Element, cat category) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	switch {
	case e.Type == "Operation":
		m.operationBody(e)
	case cat == catStateDef:
		m.stateMachineBody(e)
	case cat == catCalcDef:
		m.calcBody(e)
	case e.Type == "Interaction":
		m.parameters(e, e)
		m.interactionBody(e)
	case e.Type == "Activity":
		m.parameters(e, e)
		m.activityBody(e, e)
	default:
		m.parameters(e, e)
		m.opaqueBehaviorBody(e, e)
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// methodBehavior accounts for a behavior that is the method of an operation:
// it is written as that operation's body, not as a declaration of its own.
func (m *migration) methodBehavior(e, op *xmi.Element) {
	m.add(e, Mapped, m.v2Name(op), "written as the body of the operation "+qualifiedName(op)+", whose method it is")
}

// classifierBehavior writes the usage that performs or exhibits the behavior
// a class names as its classifier behavior, so an object of it runs it.
func (m *migration) classifierBehavior(c *xmi.Element) {
	b := m.model.Ref(c, "classifierBehavior")
	if b == nil || b.Parent != c || !m.written(b) {
		return
	}
	name := ""
	if op := m.methodOf[b]; op != nil {
		b, name = op, m.operationUsage(op)
	} else if classifierOf(b) == c {
		name = m.behaviorUsage(b)
	} else {
		name = m.freshName(c, lowerFirst(m.nameFor(b)))
	}
	cat, _ := m.classify(b)
	switch cat {
	case catStateDef:
		m.w.line("exhibit state " + writeName(name) + " : " + m.ref(b, c) + ";")
	case catActionDef:
		m.w.line("perform action " + writeName(name) + " : " + m.ref(b, c) + ";")
	default:
		return
	}
	m.downgrade(b, "the classifier behavior is run by every object of "+qualifiedName(c)+" as its usage "+name)
}

// operationUsage names the action usage of an operation's owner that performs
// it, which a call on an object refers to as `obj.<usage>`; the first ask names it.
func (m *migration) operationUsage(op *xmi.Element) string {
	if name, ok := m.opUsage[op]; ok {
		return name
	}
	name := m.freshName(op.Parent, lowerFirst(m.nameFor(op)))
	m.opUsage[op] = name
	return name
}

// operationFeature writes the action usage that makes an operation a feature of
// its owner, as a v1 operation is; the classifier behavior's own performance is that usage.
func (m *migration) operationFeature(op *xmi.Element) {
	if m.classifierBehaviorOperation(op.Parent) == op {
		return
	}
	usage := m.operationUsage(op)
	m.w.line("action " + writeName(usage) + " : " + m.ref(op, op.Parent) + ";")
	m.add(op, Mapped, "", "its owner's usage "+usage+" performs it, as a call on an object does")
}

// classifierBehaviorOperation is the operation whose method c's classifier
// behavior is, when it is written as one; nil otherwise.
func (m *migration) classifierBehaviorOperation(c *xmi.Element) *xmi.Element {
	b := m.model.Ref(c, "classifierBehavior")
	if b == nil || b.Parent != c || !m.written(b) {
		return nil
	}
	return m.methodOf[b]
}

// parameters writes the owned parameters of a behavior or operation as the
// directed parameters of the v2 definition; scope is the definition written.
func (m *migration) parameters(e, scope *xmi.Element) {
	for _, p := range e.Owned("ownedParameter") {
		m.parameter(p, scope, nil)
	}
}

// realizeParameters pairs a method's parameters with its operation's by position:
// one agreeing in direction and type stands for the operation's and takes its name.
func (m *migration) realizeParameters(op, method *xmi.Element) {
	ops := op.Owned("ownedParameter")
	for i, mp := range method.Owned("ownedParameter") {
		if i >= len(ops) {
			return
		}
		md, _ := parameterDirection(mp)
		od, _ := parameterDirection(ops[i])
		if md != od || !m.conform(m.model.Ref(mp, "type"), m.model.Ref(ops[i], "type")) {
			continue
		}
		m.realizes[mp] = ops[i]
		m.names[mp] = m.nameFor(ops[i])
	}
}

// parameter writes one parameter; declared lists the names already written in
// the definition, so a method's parameter standing for its operation's is skipped.
func (m *migration) parameter(p, scope *xmi.Element, declared map[string]bool) {
	name := m.nameOf(p)
	if name == "" {
		name = m.nameFor(p)
	}
	if op := m.realizes[p]; op != nil {
		m.add(p, Mapped, m.v2Name(op), "stands for the operation's parameter "+name+" at the same position, which is written once")
		return
	}
	dir, note := parameterDirection(p)
	if declared != nil {
		if declared[name] {
			m.add(p, Mapped, m.v2Name(p), "shares the name of the operation's parameter, which is written once")
			return
		}
		declared[name] = true
		if p.Parent != scope {
			note = joinNotes(note, "the method's parameter matches none of the operation's by position, direction and type; a call binds only the operation's parameters")
		}
	}
	t := m.model.Ref(p, "type")
	typ, tnote := m.typeRef(t, scope)
	note = joinNotes(note, tnote)
	var b strings.Builder
	b.WriteString(dir + " " + writeName(name))
	if typ != "" {
		b.WriteString(" : " + typ)
	}
	mult, mnote := m.multiplicity(p)
	b.WriteString(mult)
	note = joinNotes(note, mnote)
	var body []string
	bound, isBound := m.bound[p]
	if isBound {
		b.WriteString(" = " + bound)
	}
	if dv := firstOwned(p, "defaultValue"); dv != nil && isBound {
		note = joinNotes(note, "the default value "+describeValue(dv)+" gives way to the binding")
	} else if dv != nil {
		expr, ok, vnote := m.behaviorValue(dv, scope)
		if ok {
			b.WriteString(" default = " + expr)
			note = joinNotes(note, vnote)
		} else {
			body = commentLines("default value not migrated: " + describeValue(dv) + " — " + vnote)
			note = joinNotes(note, "default value not migrated: "+vnote)
		}
	}
	v := verdictFor(note)
	if isBound {
		note = joinNotes("bound to "+bound+", the signal the transition accepts", note)
	}
	m.add(p, v, m.v2Name(p), note)
	m.w.block(b.String(), func() {
		m.comments(p)
		m.w.lines(body)
	})
}

// parameterDirection maps a UML parameter direction to a v2 one: a return
// parameter is an out parameter, since an action def has no return.
func parameterDirection(p *xmi.Element) (string, string) {
	switch p.Attrs["direction"] {
	case "out":
		return "out", ""
	case "inout":
		return "inout", ""
	case "return":
		return "out", "the return parameter is written as an out parameter"
	}
	return "in", ""
}

// behaviorValue writes a value specification read inside a behavior: an opaque
// expression's names resolve from scope, the enclosing classifier's through `this`.
func (m *migration) behaviorValue(v, scope *xmi.Element) (expr string, ok bool, note string) {
	if v.Type != "OpaqueExpression" {
		return m.valueExpr(v, scope)
	}
	body, lang := opaqueBody(v)
	return m.behaviorExpr(body, lang, scope)
}

// typedBehaviorValue writes v as the value of feature f, read inside scope: a
// translated body must yield what f holds, and a literal, opaque or not, is
// checked against that type.
func (m *migration) typedBehaviorValue(v, f, scope *xmi.Element) (expr string, ok bool, note string) {
	if v.Type != "OpaqueExpression" {
		return m.featureValue(v, f, scope)
	}
	t := m.model.Ref(f, "type")
	sv := m.scalarBase(t)
	body, lang := opaqueBody(v)
	expr, ok, note = m.behaviorExprAs(body, lang, scope, m.wantedOf(f))
	if !ok {
		return expr, ok, note
	}
	kind, text := exprLiteral(expr)
	if kind == "" {
		return expr, ok, note
	}
	if sv == "" {
		if m.structuredValueType(t) || m.written(t) {
			return "", false, "the literal " + expr + " is not a value of " + qualifiedName(t) + ", which has no scalar base"
		}
		return expr, ok, note
	}
	value, spelled := scalarLiteral(kind, expr, text, sv)
	switch {
	case !spelled:
		return "", false, "the " + kind + " " + expr + " is not a value of " + sv + ", which the feature holds"
	case value != expr:
		return value, true, joinNotes(note, "the "+kind+" "+expr+" is written as the "+sv+" the feature holds")
	}
	return expr, ok, note
}

// behaviorExpr writes text as a v2 expression read inside scope, or refuses
// with the reason: it is not expression syntax, or a name resolves to nothing.
func (m *migration) behaviorExpr(text, lang string, scope *xmi.Element) (expr string, ok bool, note string) {
	return m.behaviorExprAs(text, lang, scope, wanted{})
}

// behaviorExprAs is behaviorExpr yielding what want asks for: a body in a
// language the translator reads is translated first, then read as v2 syntax.
func (m *migration) behaviorExprAs(text, lang string, scope *xmi.Element, want wanted) (expr string, ok bool, note string) {
	expr, ok, note, _ = m.behaviorExprHow(text, lang, scope, want)
	return expr, ok, note
}

// behaviorExprHow is behaviorExprAs also reporting whether the translator
// wrote the expression, rather than the body being v2 syntax already.
func (m *migration) behaviorExprHow(text, lang string, scope *xmi.Element, want wanted) (expr string, ok bool, note string, translated bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false, "the expression has no body", false
	}
	var refused *refusal
	if dialectOf(lang) != dialectNone {
		expr, note, refused = m.translatedExpr(text, lang, scope, want)
		if refused == nil {
			m.noted(scope, note)
			return expr, true, "", true
		}
		if refused.final(lang) {
			return "", false, refused.note(), false
		}
	}
	expr, ok, note = m.v2Expr(text, lang, scope)
	if !ok {
		return "", false, refusedNote(refused, note, lang), false
	}
	return expr, true, note, false
}

// v2Expr writes text, already v2 expression syntax, read inside scope, or
// refuses with the reason: it is not expression syntax, or a name resolves to nothing.
func (m *migration) v2Expr(text, lang string, scope *xmi.Element) (expr string, ok bool, note string) {
	refs, ok := exprRefs(text)
	if !ok {
		return "", false, "not v2 expression syntax" + langNote(lang)
	}
	if missing := m.invisible(refs, scope); missing != "" {
		return "", false, missing + langNote(lang)
	}
	return m.qualifySelf(text, refs, scope), true, ""
}

// refusedNote is the note for a body neither translated nor read as v2: the
// translator's refusal when the body declares a language it reads, else the
// v2 reading's (a body declaring no language is v2 first).
func refusedNote(refused *refusal, v2Note, lang string) string {
	if refused == nil || refused.kind == refusedLanguage || strings.TrimSpace(lang) == "" {
		return v2Note
	}
	return refused.note()
}

// qualifySelf prefixes `this.` to each name in text that resolves to a feature
// of the classifier enclosing scope, which a nested action reaches no other way.
func (m *migration) qualifySelf(text string, refs []reference, scope *xmi.Element) string {
	visible, _ := m.visibleFrom(scope)
	var starts []int
	for _, r := range refs {
		if r.global || r.local != "" || len(r.steps) == 0 {
			continue
		}
		f := visible[r.steps[0].name]
		if f == nil || (f.Type != "Property" && f.Type != "Port") || !m.ownedByClassifier(f, scope) {
			continue
		}
		starts = append(starts, r.start)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(starts)))
	for _, s := range starts {
		text = text[:s] + "this." + text[s:]
	}
	return text
}

// ownedByClassifier reports whether feature f belongs to a classifier that
// scope's behavior is written inside, rather than to the behavior itself.
func (m *migration) ownedByClassifier(f, scope *xmi.Element) bool {
	owner := f.Parent
	if owner == nil {
		return false
	}
	cur := scope
	for ; cur != nil && !isBehavior(cur) && cur.Type != "Operation"; cur = cur.Parent {
		if cur == owner {
			return false
		}
	}
	for ; cur != nil; cur = cur.Parent {
		if cur == owner {
			return !behaviorScope(cur)
		}
		if !behaviorScope(cur) {
			return m.inherits(cur, owner)
		}
	}
	return false
}

// behaviorScope reports whether `this` reaches through e to the classifier
// enclosing it: a behavior, an operation or a piece of a state machine.
func behaviorScope(e *xmi.Element) bool {
	switch e.Type {
	case "Operation", "Region", "State", "Transition", "Pseudostate", "FinalState":
		return true
	}
	return isBehavior(e)
}

// assignment is one statement of an opaque body written as a v2 assignment.
var assignment = regexp.MustCompile(`^([\p{L}_][\p{L}\p{N}_ ]*)\s*(\+\+|--|[-+*/]?=)\s*(.*)$`)

// statements writes an opaque body as v2 assignments read inside scope: a body
// in a language the translator reads is translated first, then each statement
// is read as an assignment of a v2 expression; else refuses with the reason.
func (m *migration) statements(body, lang string, scope *xmi.Element) (lines []string, ok bool, note string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, false, "the body is empty"
	}
	var refused *refusal
	if dialectOf(lang).script() {
		lines, note, refused = m.translatedStatements(body, lang, scope)
		if refused == nil {
			m.noted(scope, note)
			return lines, true, ""
		}
		if refused.final(lang) {
			return nil, false, refused.note()
		}
	}
	lines, ok, note = m.v2Statements(body, lang, scope)
	if !ok {
		return nil, false, refusedNote(refused, note, lang)
	}
	return lines, true, note
}

// v2Statements writes an opaque body whose every statement assigns a v2
// expression to a visible feature, else refuses with the reason.
func (m *migration) v2Statements(body, lang string, scope *xmi.Element) (lines []string, ok bool, note string) {
	if strings.ContainsAny(body, "{}") {
		return nil, false, "the body is not a sequence of assignments" + langNote(lang)
	}
	for _, st := range strings.Split(body, ";") {
		st = strings.TrimSpace(st)
		if st == "" {
			continue
		}
		mt := assignment.FindStringSubmatch(st)
		if mt == nil || strings.HasPrefix(mt[3], "=") {
			return nil, false, "the statement " + strconv.Quote(st) + " is not an assignment of a v2 expression" + langNote(lang)
		}
		lhs, op, rhs := strings.TrimSpace(mt[1]), mt[2], strings.TrimSpace(mt[3])
		target, ok := m.assignable(lhs, scope)
		if !ok {
			return nil, false, "the statement assigns " + lhs + ", which nothing visible from " + qualifiedName(scope) + " is called" + langNote(lang)
		}
		switch op {
		case "++", "--":
			if rhs != "" {
				return nil, false, "the statement " + strconv.Quote(st) + " is not an assignment" + langNote(lang)
			}
			lines = append(lines, "assign "+target+" := "+target+" "+op[:1]+" 1;")
			continue
		}
		expr, ok, enote := m.v2Expr(rhs, lang, scope)
		if !ok {
			return nil, false, "the statement " + strconv.Quote(st) + " assigns a value whose expression is not migrated: " + enote
		}
		if op != "=" {
			expr = target + " " + op[:1] + " (" + expr + ")"
		}
		lines = append(lines, "assign "+target+" := "+expr+";")
	}
	if len(lines) == 0 {
		return nil, false, "the body is empty"
	}
	return lines, true, "the " + langName(lang) + " body is written as v2 assignments"
}

// assignable writes the v2 target of an assignment to name read in scope: a
// parameter or local of the behavior bare, a feature of its classifier as `this.`.
func (m *migration) assignable(name string, scope *xmi.Element) (string, bool) {
	visible, _ := m.visibleFrom(scope)
	f := visible[name]
	if f == nil {
		return "", false
	}
	switch f.Type {
	case "Parameter", "Property", "Port":
	default:
		return "", false
	}
	if m.ownedByClassifier(f, scope) {
		return "this." + writeName(name), true
	}
	return writeName(name), true
}

// durationUnits scales each unit a v1 duration literal may carry to seconds.
var durationUnits = map[string]float64{
	"": 1, "s": 1, "sec": 1, "secs": 1, "second": 1, "seconds": 1,
	"ms": 1e-3, "millisecond": 1e-3, "milliseconds": 1e-3,
	"us": 1e-6, "µs": 1e-6, "microsecond": 1e-6, "microseconds": 1e-6,
	"min": 60, "mins": 60, "minute": 60, "minutes": 60,
	"h": 3600, "hr": 3600, "hrs": 3600, "hour": 3600, "hours": 3600,
	"d": 86400, "day": 86400, "days": 86400,
}

var (
	durationTerm     = regexp.MustCompile(`^([0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?)\s*([\p{L}µ]*)`)
	durationVariable = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*\s*=\s*`)
)

// parseDuration reads a duration literal such as `1s`, `0.5 s`, `80ms`, `2 min` or
// `t = 1 minute 30 seconds` as a number of seconds written as a v2 real literal.
func parseDuration(text string) (seconds string, ok bool) {
	rest := durationVariable.ReplaceAllString(strings.TrimSpace(text), "")
	if rest == "" {
		return "", false
	}
	var total float64
	for terms := 0; rest != ""; terms++ {
		mt := durationTerm.FindStringSubmatch(rest)
		if mt == nil {
			return "", false
		}
		v, err := strconv.ParseFloat(mt[1], 64)
		if err != nil {
			return "", false
		}
		scale, known := durationUnits[strings.ToLower(mt[2])]
		if !known || (mt[2] == "" && terms > 0) {
			return "", false
		}
		total += v * scale
		rest = strings.TrimSpace(rest[len(mt[0]):])
	}
	if math.IsInf(total, 0) || math.IsNaN(total) {
		return "", false
	}
	return realLiteral(total), true
}

// realLiteral writes a float as a v2 real literal, with a decimal point.
func realLiteral(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// durationExpr writes a UML duration value as a v2 expression in seconds: a scaled
// literal, a bare number, or an expression read in scope; else ok is false with why.
func (m *migration) durationExpr(v, scope *xmi.Element) (expr string, ok bool, note string) {
	for v != nil && (v.Type == "Duration" || v.Type == "TimeExpression") {
		v = firstOwned(v, "expr")
	}
	if v == nil {
		return "", false, "the duration has no expression"
	}
	switch v.Type {
	case "LiteralString":
		if s, ok := parseDuration(v.Attrs["value"]); ok {
			return s, true, ""
		}
		expr, ok, note := m.symbolicDuration(v.Attrs["value"], "", scope)
		if !ok {
			return "", false, "the duration " + strconv.Quote(v.Attrs["value"]) + " is neither a number with a time unit nor an expression: " + note
		}
		return expr, true, note
	case "LiteralInteger", "LiteralReal", "LiteralUnlimitedNatural":
		expr, ok, note := m.valueExpr(v, scope)
		if !ok {
			return "", false, note
		}
		if s, ok := parseDuration(expr); ok {
			return s, true, "the duration " + expr + " carries no unit and is taken as seconds"
		}
		return "", false, "the duration " + expr + " is not a finite number"
	case "OpaqueExpression":
		body, lang := opaqueBody(v)
		if s, ok := parseDuration(body); ok {
			return s, true, ""
		}
		expr, ok, note := m.symbolicDuration(body, lang, scope)
		if !ok {
			return "", false, "the duration " + strconv.Quote(body) + " is neither a number with a time unit nor an expression: " + note
		}
		return expr, true, note
	}
	return "", false, "a UML " + v.Type + " has no v2 duration form"
}

// calcExpr returns the result expression of an opaque or function behavior,
// when its one body is a v2 expression whose names resolve from the behavior.
func (m *migration) calcExpr(e *xmi.Element) (expr string, ok bool, note string) {
	expr, ok, note, _ = m.calcExprHow(e)
	return expr, ok, note
}

// calcExprHow is calcExpr also reporting whether the translator wrote the expression.
func (m *migration) calcExprHow(e *xmi.Element) (expr string, ok bool, note string, translated bool) {
	bodies := e.Owned("body")
	if len(bodies) > 1 {
		return "", false, "the behavior has " + strconv.Itoa(len(bodies)) + " bodies; only one can be the result expression", false
	}
	body, lang := opaqueBody(e)
	if body == "" {
		return "", false, "the behavior has no body", false
	}
	want, rnote := m.calcResult(e)
	if rnote != "" {
		return "", false, rnote, false
	}
	return m.behaviorExprHow(body, lang, e, want)
}

// calcResult is what a behavior's result expression must yield: the value of
// its one return or output parameter; a behavior with none wants any value,
// and with several the expression can stand for none of them.
func (m *migration) calcResult(e *xmi.Element) (wanted, string) {
	var outs []*xmi.Element
	for _, p := range e.Owned("ownedParameter") {
		if dir, _ := parameterDirection(p); dir != "in" {
			outs = append(outs, p)
		}
	}
	switch len(outs) {
	case 0:
		return wanted{}, ""
	case 1:
		return m.wantedOf(outs[0]), ""
	}
	return wanted{}, "the behavior has " + strconv.Itoa(len(outs)) + " output parameters; one result expression can stand for none of them"
}

// resultRefusal is why a body the translator reads whole as an expression is
// still no result expression for e: its type, or which parameter it would be.
func (m *migration) resultRefusal(e *xmi.Element) string {
	body, lang := opaqueBody(e)
	if dialectOf(lang) == dialectNone {
		return ""
	}
	want, rnote := m.calcResult(e)
	_, _, err := m.translatedExpr(body, lang, e, want)
	switch {
	case err == nil:
		return rnote
	case err.kind == refusedType:
		return err.note()
	}
	return ""
}

// calcBody writes an opaque or function behavior's parameters and result expression.
func (m *migration) calcBody(e *xmi.Element) {
	m.parameters(e, e)
	expr, _, _, translated := m.calcExprHow(e)
	_, lang := opaqueBody(e)
	m.w.line(expr)
	if lang != "" && !translated {
		m.downgrade(e, "the "+lang+" body is written verbatim as the result expression, since it is also v2 expression syntax")
	}
}

// opaqueBehaviorBody writes an opaque or function behavior's body that is no single
// expression: as assignments when every statement is one, else as a comment.
func (m *migration) opaqueBehaviorBody(e, scope *xmi.Element) {
	body, lang := opaqueBody(e)
	lines, ok, note := m.statements(body, lang, scope)
	if !ok {
		if r := m.resultRefusal(e); r != "" && r != note {
			note = "as the result expression, " + r + "; as statements, " + note
		}
		m.opaqueComment(body, lang, note)
		m.downgrade(e, "the body is kept as a comment: "+note)
		return
	}
	name := m.freshName(scope, "body")
	m.w.line("first start then " + writeName(name) + ";")
	m.w.block("action "+writeName(name), func() { m.w.lines(lines) })
	m.w.line("first " + writeName(name) + " then done;")
	if note != "" {
		m.downgrade(e, note)
	}
}

// opaqueComment keeps an opaque body the mapping cannot write as a comment.
func (m *migration) opaqueComment(body, lang, note string) {
	text := "body not migrated"
	if note != "" {
		text += " (" + note + ")"
	}
	if lang != "" {
		text += " {" + lang + "}"
	}
	if body != "" {
		text += ":\n" + body
	}
	m.w.lines(commentLines(text))
}

func langName(lang string) string {
	if lang == "" {
		return "opaque"
	}
	return lang
}

// operationBody writes an operation's parameters, conditions and method.
func (m *migration) operationBody(op *xmi.Element) {
	declared := map[string]bool{}
	for _, p := range op.Owned("ownedParameter") {
		m.parameter(p, op, declared)
	}
	method := m.model.Ref(op, "method")
	if method != nil {
		for _, p := range method.Owned("ownedParameter") {
			m.parameter(p, op, declared)
		}
	}
	m.operationConditions(op)
	switch {
	case method == nil:
		if len(m.model.Unresolved(op, "method")) > 0 {
			m.downgrade(op, "the method refers to nothing in the document; the operation is written abstract")
		}
	case method.Parent != op.Parent:
		m.downgrade(op, "the method "+qualifiedName(method)+" is owned elsewhere and written there; the operation is written abstract")
	case method.Type == "Activity":
		m.activityBody(method, op)
	case method.Type == "OpaqueBehavior" || method.Type == "FunctionBehavior":
		m.opaqueBehaviorBody(method, op)
	default:
		m.downgrade(op, "a "+method.Type+" method has no action body form")
	}
}

// abstractOperation reports whether an operation is written abstract: it has
// no method of its own to become its body.
func (m *migration) abstractOperation(op *xmi.Element) bool {
	if op.Type != "Operation" {
		return false
	}
	method := m.model.Ref(op, "method")
	return method == nil || method.Parent != op.Parent
}

// operationConditions writes an operation's pre-, post- and body conditions
// as asserted constraints when they are v2 expressions, else as comments.
func (m *migration) operationConditions(op *xmi.Element) {
	for _, role := range []string{"precondition", "postcondition", "bodyCondition"} {
		for _, c := range m.model.Refs(op, role) {
			spec := firstOwned(c, "specification")
			if spec == nil {
				m.unmapped(c, "the "+role+" has no specification")
				continue
			}
			expr, ok, note := m.behaviorValue(spec, op)
			if !ok {
				m.w.lines(commentLines(role + " not migrated: " + describeValue(spec) + " — " + note))
				m.add(c, Unmapped, "", note)
				continue
			}
			decl := "assert constraint"
			if m.nameOf(c) != "" {
				decl += " " + writeName(m.nameOf(c))
			}
			m.w.line(decl + " { " + expr + " }")
			m.add(c, verdictFor(note), m.v2Name(c), joinNotes("the "+role+" is asserted over the action", note))
		}
	}
	for _, r := range op.Owned("ownedRule") {
		if m.reported(r) {
			continue
		}
		m.rule(r)
	}
}

// reported says whether e already has a report entry.
func (m *migration) reported(e *xmi.Element) bool {
	_, ok := m.indexed[e.ID]
	return ok
}

// reception writes a reception as a comment on its owner: v2 has no
// reception, the signal it names being accepted by the owner's behaviors.
func (m *migration) reception(r *xmi.Element) {
	sig := m.model.Ref(r, "signal")
	text := "reception " + describe(r)
	note := "a reception names the signal its owner accepts, which the owner's behaviors carry as accept"
	switch {
	case sig == nil:
		note = joinNotes(note, m.dangling(r, "signal"))
		if note == "" || len(m.model.Unresolved(r, "signal")) == 0 {
			note = joinNotes(note, "the reception names no signal")
		}
	case m.written(sig):
		text += " accepts " + m.ref(sig, m.scope)
	default:
		text += " accepts " + qualifiedName(sig)
		note = joinNotes(note, "the signal "+qualifiedName(sig)+" has no v2 declaration in the document")
	}
	m.w.lines(prefixFirst(commentPrefix, commentLines(text)))
	m.add(r, Approximated, "", note)
}

// event reports an event declared as a member: it is written as an accept clause
// where a trigger refers to it, and the triggers report those, in their own scope.
func (m *migration) event(e *xmi.Element) {
	if m.triggered[e] {
		return
	}
	clause, note, ok := m.acceptClause(e, e.Parent, "")
	if !ok {
		m.unmapped(e, joinNotes("no trigger refers to the event", note))
		return
	}
	m.add(e, Approximated, "", joinNotes("no trigger refers to the event, which would be written as "+clause, note))
}
