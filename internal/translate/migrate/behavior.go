package migrate

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// stmtNote and durNote open the diagnostics statements and durations repeat.
const (
	stmtNote = "the statement "
	durNote  = "the duration "
)

// classifyBehavior decides the v2 declaration a UML behavior becomes: action def,
// state def, calc def for an expression body, or a scenario action def for an interaction.

// The note fragments the writer repeats.
const (
	methodNote = "the method "
)

func (m *migration) classifyBehavior(e *sysmlv1.Element) (category, string) {
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
func isBehavior(e *sysmlv1.Element) bool {
	switch e.Type {
	case "Activity", "StateMachine", "OpaqueBehavior", "FunctionBehavior", "Interaction":
		return true
	}
	return false
}

// behaviorBody writes the body of a behavior or operation declaration.
func (m *migration) behaviorBody(e *sysmlv1.Element, cat category) {
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
		m.contextParameter(e)
		m.activityBody(e, e)
	default:
		m.parameters(e, e)
		m.opaqueBehaviorBody(e, e)
	}
	m.stereotypeAnnotations(e)
	m.scope = saved
}

// methodBehavior accounts for a behavior that is the method of an operation:
// it is written as that operation's body, not as a declaration of its own.
func (m *migration) methodBehavior(e, op *sysmlv1.Element) {
	m.add(e, Mapped, m.v2Name(op), "written as the body of the operation "+qualifiedName(op)+", whose method it is")
}

// classifierBehavior writes the usage that performs or exhibits the behavior
// a class names as its classifier behavior, so an object of it runs it.
func (m *migration) classifierBehavior(c *sysmlv1.Element) {
	b, name, cat := m.classifierBehaviorUsage(c)
	if b == nil {
		return
	}
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

// classifierBehaviorUsage names the usage by which an object of c runs its
// classifier behavior, and the behavior (or the operation it is the method of)
// with its category; nil when c has no written classifier behavior of its own.
// The name is fixed on the first ask, so a reference (a call on an object, a
// swimlane's performer) may precede the declaration.
func (m *migration) classifierBehaviorUsage(c *sysmlv1.Element) (b *sysmlv1.Element, name string, cat category) {
	b = m.model.Ref(c, "classifierBehavior")
	if b == nil || b.Parent != c || !m.written(b) {
		return nil, "", catNone
	}
	if op := m.methodOf[b]; op != nil {
		b, name = op, m.operationUsage(op)
	} else {
		name = m.behaviorUsage(b)
	}
	cat, _ = m.classify(b)
	return b, name, cat
}

// operationUsage names the action usage of an operation's owner that performs
// it, which a call on an object refers to as `obj.<usage>`; the first ask names it.
func (m *migration) operationUsage(op *sysmlv1.Element) string {
	if name, ok := m.opUsage[op]; ok {
		return name
	}
	name := m.freshName(op.Parent, lowerFirst(m.nameFor(op)))
	m.opUsage[op] = name
	return name
}

// operationFeature writes the action usage that makes an operation a feature of
// its owner, as a v1 operation is; the classifier behavior's own performance is that usage.
func (m *migration) operationFeature(op *sysmlv1.Element) {
	if m.classifierBehaviorOperation(op.Parent) == op {
		return
	}
	usage := m.operationUsage(op)
	m.w.line(actionKw + writeName(usage) + " : " + m.ref(op, op.Parent) + ";")
	m.add(op, Mapped, "", "its owner's usage "+usage+" performs it, as a call on an object does")
}

// classifierBehaviorOperation is the operation whose method c's classifier
// behavior is, when it is written as one; nil otherwise.
func (m *migration) classifierBehaviorOperation(c *sysmlv1.Element) *sysmlv1.Element {
	b := m.model.Ref(c, "classifierBehavior")
	if b == nil || b.Parent != c || !m.written(b) {
		return nil
	}
	return m.methodOf[b]
}

// parameters writes the owned parameters of a behavior or operation as the
// directed parameters of the v2 definition; scope is the definition written.
func (m *migration) parameters(e, scope *sysmlv1.Element) {
	for _, p := range e.Owned("ownedParameter") {
		m.parameter(p, scope, nil)
	}
}

// realizeParameters pairs a method's parameters with its operation's by position:
// one agreeing in direction and type stands for the operation's and takes its name.
func (m *migration) realizeParameters(op, method *sysmlv1.Element) {
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
func (m *migration) parameter(p, scope *sysmlv1.Element, declared map[string]bool) {
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
			note = joinNotes(note, "the method's parameter matches none of the operation's by position, direction and type; it is declared after them, and a call binds it there")
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
		expr, ok, vnote := m.typedBehaviorValue(dv, p, scope)
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
		what := m.boundNote
		if what == "" {
			what = "the signal the transition accepts"
		}
		note = joinNotes("bound to "+bound+", "+what, note)
	}
	m.add(p, v, m.v2Name(p), note)
	m.w.block(b.String(), func() {
		m.comments(p)
		m.w.lines(body)
	})
}

// parameterDirection maps a UML parameter direction to a v2 one: a return
// parameter is an out parameter, since an action def has no return.
func parameterDirection(p *sysmlv1.Element) (string, string) {
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
func (m *migration) behaviorValue(v, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	if v.Type != "OpaqueExpression" {
		return m.valueExpr(v, scope)
	}
	body, lang := opaqueBody(v)
	return m.behaviorExpr(body, lang, scope)
}

// typedBehaviorValue writes v as the value of feature f, read inside scope: a
// translated body must yield what f holds, and a literal, opaque or not, is
// checked against that type.
func (m *migration) typedBehaviorValue(v, f, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	if v.Type != "OpaqueExpression" {
		return m.featureValue(v, f, scope)
	}
	t := m.model.Ref(f, "type")
	body, lang := opaqueBody(v)
	expr, ok, note = m.behaviorExprAs(body, lang, scope, m.wantedOf(f))
	if !ok {
		return expr, ok, note
	}
	if kind, _ := exprLiteral(expr); kind != "" && m.scalarBase(t) == "" && (m.structuredValueType(t) || m.written(t)) {
		return "", false, "the literal " + expr + " is not a value of " + qualifiedName(t) + ", which has no scalar base"
	}
	return expr, ok, note
}

// behaviorExpr writes text as a v2 expression read inside scope, or refuses
// with the reason: it is not expression syntax, or a name resolves to nothing.
func (m *migration) behaviorExpr(text, lang string, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	return m.behaviorExprAs(text, lang, scope, wanted{})
}

// behaviorExprAs is behaviorExpr yielding what want asks for: a body in a
// language the translator reads is translated first, then read as v2 syntax.
func (m *migration) behaviorExprAs(text, lang string, scope *sysmlv1.Element, want wanted) (expr string, ok bool, note string) {
	expr, ok, note, _ = m.behaviorExprHow(text, lang, scope, want)
	return expr, ok, note
}

// behaviorExprHow is behaviorExprAs also reporting whether the translator
// wrote the expression, rather than the body being v2 syntax already. An
// expression that is one literal is checked to spell a value of the wanted scalar.
func (m *migration) behaviorExprHow(text, lang string, scope *sysmlv1.Element, want wanted) (expr string, ok bool, note string, translated bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false, "the expression has no body", false
	}
	if dialectOf(lang) != dialectNone {
		expr, note, refused := m.translatedExpr(text, lang, scope, want)
		if refused == nil {
			m.noted(scope, note)
			expr, ok, note = literalExprAs(expr, want)
			return expr, ok, note, true
		}
		if refused.final(lang) {
			return "", false, refused.note(), false
		}
	}
	expr, ok, note = m.v2Expr(text, lang, scope)
	if !ok {
		return "", false, note, false
	}
	expr, ok, lnote := literalExprAs(expr, want)
	return expr, ok, joinNotes(note, lnote), false
}

// literalExprAs writes expr as the value want asks for when it is one literal,
// and as it is otherwise.
func literalExprAs(expr string, want wanted) (value string, ok bool, note string) {
	kind, text := exprLiteral(expr)
	if kind == "" {
		return expr, true, ""
	}
	return literalAs(kind, expr, text, want)
}

// v2Expr writes text, already v2 expression syntax, read inside scope, or
// refuses with the reason: it is not expression syntax, or a name resolves to nothing.
func (m *migration) v2Expr(text, lang string, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	refs, ok := exprRefs(text)
	if !ok {
		return "", false, "not v2 expression syntax" + langNote(lang)
	}
	if missing := m.invisible(refs, scope); missing != "" {
		return "", false, missing + langNote(lang)
	}
	return m.qualifySelf(text, refs, scope), true, ""
}

// qualifySelf prefixes `this.` (or the subject's name, in a test case) to each name
// in text that resolves to a feature of the classifier enclosing scope.
func (m *migration) qualifySelf(text string, refs []reference, scope *sysmlv1.Element) string {
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
		text = text[:s] + m.self + "." + text[s:]
	}
	return text
}

// ownedByClassifier reports whether feature f belongs to a classifier that
// scope's behavior is written inside, rather than to the behavior itself.
func (m *migration) ownedByClassifier(f, scope *sysmlv1.Element) bool {
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
func behaviorScope(e *sysmlv1.Element) bool {
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
func (m *migration) statements(body, lang string, scope *sysmlv1.Element) (lines []string, ok bool, note string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, false, "the body is empty"
	}
	if dialectOf(lang).script() {
		lines, note, otherwise, refused := m.translatedStatements(body, lang, scope)
		if refused == nil {
			m.noted(scope, note)
			m.notedAs(scope, Approximated, otherwise)
			return lines, true, ""
		}
		if refused.final(lang) {
			return nil, false, refused.note()
		}
	}
	return m.v2Statements(body, lang, scope)
}

// v2Statements writes an opaque body whose every statement assigns a v2
// expression to a visible feature, else refuses with the reason.
func (m *migration) v2Statements(body, lang string, scope *sysmlv1.Element) (lines []string, ok bool, note string) {
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
			return nil, false, stmtNote + strconv.Quote(st) + " is not an assignment of a v2 expression" + langNote(lang)
		}
		lhs, op, rhs := strings.TrimSpace(mt[1]), mt[2], strings.TrimSpace(mt[3])
		target, ok := m.assignable(lhs, scope)
		if !ok {
			return nil, false, "the statement assigns " + lhs + ", which nothing visible from " + qualifiedName(scope) + " is called" + langNote(lang)
		}
		switch op {
		case "++", "--":
			if rhs != "" {
				return nil, false, stmtNote + strconv.Quote(st) + " is not an assignment" + langNote(lang)
			}
			lines = append(lines, "assign "+target+" := "+target+" "+op[:1]+" 1;")
			continue
		}
		expr, ok, enote := m.v2Expr(rhs, lang, scope)
		if !ok {
			return nil, false, stmtNote + strconv.Quote(st) + " assigns a value whose expression is not migrated: " + enote
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
func (m *migration) assignable(name string, scope *sysmlv1.Element) (string, bool) {
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

// durationUnits scales a v1 duration's unit to seconds, spelled as the simulation toolkit
// spells the units of fixed length (plus `secs`, `mins`, `us`); none is its default, the millisecond.
var durationUnits = map[string]float64{
	"": 1e-3, "ms": 1e-3, "millisec": 1e-3, "millisecond": 1e-3, "milliseconds": 1e-3,
	"s": 1, "sec": 1, "secs": 1, "second": 1, "seconds": 1,
	"us": 1e-6, "µs": 1e-6, "microsec": 1e-6, "microsecond": 1e-6, "microseconds": 1e-6,
	"ns": 1e-9, "nsec": 1e-9, "nanosecond": 1e-9, "nanoseconds": 1e-9,
	"m": 60, "min": 60, "mins": 60, "minute": 60, "minutes": 60,
	"h": 3600, "hr": 3600, "hrs": 3600, "hour": 3600, "hours": 3600,
	"d": 86400, "day": 86400, "days": 86400,
	"wk": 604800, "week": 604800, "weeks": 604800,
}

// bareDurationNote says how a duration with no unit is read.
const bareDurationNote = " carries no unit and is read in milliseconds, the simulation toolkit's default"

var (
	durationTerm     = regexp.MustCompile(`^([0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?)\s*([\p{L}µ]*)`)
	durationVariable = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*\s*=\s*`)
)

// parseDuration reads a duration literal such as `1s`, `0.5 s`, `80ms`, `2 min`, `200`
// (bare: milliseconds) or `t = 1 minute 30 seconds` as seconds written as a v2 real literal.
func parseDuration(text string) (seconds string, bare, ok bool) {
	rest := durationVariable.ReplaceAllString(strings.TrimSpace(text), "")
	if rest == "" {
		return "", false, false
	}
	var total float64
	for terms := 0; rest != ""; terms++ {
		mt := durationTerm.FindStringSubmatch(rest)
		if mt == nil {
			return "", false, false
		}
		v, err := strconv.ParseFloat(mt[1], 64)
		if err != nil {
			return "", false, false
		}
		scale, known := durationUnits[strings.ToLower(mt[2])]
		if !known || (mt[2] == "" && terms > 0) {
			return "", false, false
		}
		bare = mt[2] == ""
		total += v * scale
		rest = strings.TrimSpace(rest[len(mt[0]):])
	}
	if math.IsInf(total, 0) || math.IsNaN(total) {
		return "", false, false
	}
	return computedLiteral(total), bare, true
}

// computedLiteral writes the result of arithmetic as a v2 real literal at 15
// significant digits, so the binary rounding noise of the arithmetic is not written.
func computedLiteral(v float64) string {
	if rounded, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 15, 64), 64); err == nil {
		v = rounded
	}
	return realLiteral(v)
}

// realLiteral writes a float as a v2 real literal, with a decimal point.
func realLiteral(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// singleValue is the min of a duration interval whose max is a duration without
// an expression in a MagicDraw document, which is how that tool stores and shows
// a constraint written with one value, `{60s}`; ok is false for anything else.
func (m *migration) singleValue(spec *sysmlv1.Element, lo string, lok bool, hok bool) (bound, note string, ok bool) {
	if !lok || hok || !m.fromMagicDraw() {
		return "", "", false
	}
	v := m.model.Ref(spec, "max")
	if v == nil || v.Type != "Duration" && v.Type != "TimeExpression" {
		return "", "", false
	}
	for v != nil && (v.Type == "Duration" || v.Type == "TimeExpression") {
		v = firstOwned(v, "expr")
	}
	if v != nil {
		return "", "", false
	}
	return lo, "the max is a duration without an expression, MagicDraw's form of the one-valued constraint {" + lo + " s}", true
}

// fromMagicDraw reports whether MagicDraw or Cameo wrote the document, by the
// exporter it names or the extender of its tool-private extensions.
func (m *migration) fromMagicDraw() bool {
	tool := func(s string) bool {
		s = strings.ToLower(s)
		return strings.Contains(s, "magicdraw") || strings.Contains(s, "cameo")
	}
	if tool(m.model.Exporter) {
		return true
	}
	for _, ext := range m.model.Extensions {
		if tool(ext.Extender) {
			return true
		}
	}
	return false
}

// openInterval says why an interval lacking a usable bound is not written as a
// wait: open on one side, it admits every wait past the bound it has.
func (m *migration) openInterval(spec *sysmlv1.Element, lo string, lok bool, lnote, hi string, hok bool, hnote string) string {
	missing := func(role, note string) string {
		if m.model.Ref(spec, role) == nil {
			return "the interval has no " + role
		}
		return "the interval's " + role + " is not written: " + note
	}
	switch {
	case lok && !hok:
		return missing("max", hnote) + ", so the interval is open above and no one wait of at least " + lo + " s stands for it"
	case hok && !lok:
		return missing("min", lnote) + ", so the interval is open below and no one wait of at most " + hi + " s stands for it"
	}
	return joinNotes(missing("min", lnote), missing("max", hnote))
}

// durationExpr writes a UML duration value as a v2 expression in seconds: a scaled
// literal, a bare number, or an expression read in scope; else ok is false with why.
func (m *migration) durationExpr(v, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	for v != nil && (v.Type == "Duration" || v.Type == "TimeExpression") {
		v = firstOwned(v, "expr")
	}
	if v == nil {
		return "", false, "the duration has no expression"
	}
	switch v.Type {
	case "LiteralString":
		if s, bare, ok := parseDuration(v.Attrs["value"]); ok {
			return s, true, literalDurationNote(v.Attrs["value"], bare)
		}
		expr, ok, note := m.symbolicDuration(v.Attrs["value"], "", scope)
		if !ok {
			return "", false, durNote + strconv.Quote(v.Attrs["value"]) + " is neither a number with a time unit nor an expression: " + note
		}
		return expr, true, note
	case "LiteralInteger", "LiteralReal", "LiteralUnlimitedNatural":
		if v.Type == "LiteralUnlimitedNatural" && v.Attrs["value"] == "*" {
			return "", false, "the duration * is unbounded"
		}
		expr, ok, note := m.valueExpr(v, scope)
		if !ok {
			return "", false, note
		}
		if s, _, ok := parseDuration(expr); ok {
			return s, true, durNote + expr + bareDurationNote
		}
		return "", false, durNote + expr + " is not a finite number"
	case "OpaqueExpression":
		body, lang := opaqueBody(v)
		if s, bare, ok := parseDuration(body); ok {
			return s, true, literalDurationNote(body, bare)
		}
		expr, ok, note := m.symbolicDuration(body, lang, scope)
		if !ok {
			return "", false, durNote + strconv.Quote(body) + " is neither a number with a time unit nor an expression: " + note
		}
		return expr, true, note
	}
	return "", false, "a UML " + v.Type + " has no v2 duration form"
}

// literalDurationNote notes a duration literal read with no unit; a unit needs none.
func literalDurationNote(text string, bare bool) string {
	if !bare {
		return ""
	}
	return durNote + strconv.Quote(strings.TrimSpace(text)) + bareDurationNote
}

// calcExpr returns the result expression of an opaque or function behavior,
// when its one body is a v2 expression whose names resolve from the behavior.
func (m *migration) calcExpr(e *sysmlv1.Element) (expr string, ok bool, note string) {
	expr, ok, note, _ = m.calcExprHow(e)
	return expr, ok, note
}

// calcExprHow is calcExpr also reporting whether the translator wrote the expression.
func (m *migration) calcExprHow(e *sysmlv1.Element) (expr string, ok bool, note string, translated bool) {
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
func (m *migration) calcResult(e *sysmlv1.Element) (wanted, string) {
	var outs []*sysmlv1.Element
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
func (m *migration) resultRefusal(e *sysmlv1.Element) string {
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
func (m *migration) calcBody(e *sysmlv1.Element) {
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
func (m *migration) opaqueBehaviorBody(e, scope *sysmlv1.Element) {
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
	m.w.block(actionKw+writeName(name), func() { m.w.lines(lines) })
	m.w.line(firstKw + writeName(name) + " then done;")
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

// operationBody writes an operation's parameters, conditions and method; the
// parameters are those actionParameters lists, so a call binds what is declared.
func (m *migration) operationBody(op *sysmlv1.Element) {
	declared := map[string]bool{}
	for _, p := range op.Owned("ownedParameter") {
		m.parameter(p, op, declared)
	}
	method := m.model.Ref(op, "method")
	if method != nil && method.Parent == op.Parent {
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
		m.downgrade(op, methodNote+qualifiedName(method)+" is owned elsewhere and written there; the operation is written abstract")
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
func (m *migration) abstractOperation(op *sysmlv1.Element) bool {
	if op.Type != "Operation" {
		return false
	}
	method := m.model.Ref(op, "method")
	return method == nil || method.Parent != op.Parent
}

// operationConditions writes an operation's pre-, post- and body conditions
// as asserted constraints when they are v2 expressions, else as comments.
func (m *migration) operationConditions(op *sysmlv1.Element) {
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
func (m *migration) reported(e *sysmlv1.Element) bool {
	_, ok := m.indexed[e.ID]
	return ok
}

// reception writes a reception as an action def of its owner that accepts the signal, from the
// object and via each port it arrives at, performs the method and accepts again, performed from creation.
func (m *migration) reception(r *sysmlv1.Element) {
	sig := m.model.Ref(r, "signal")
	if sig == nil || !m.written(sig) {
		m.receptionComment(r, sig)
		return
	}
	owner := r.Parent
	name := m.nameFor(r)
	usage := m.freshName(owner, lowerFirst(name))
	ports, info := m.arrivalRoutes(owner, sig, "reception")
	method := m.model.Ref(r, "method")
	performed := method
	if op := m.methodOf[method]; op != nil {
		performed = op
	}
	route := receptionRoute{owner: owner, sig: sig, method: performed, used: map[string]bool{"start": true, "done": true}}
	m.w.block("action def "+writeName(name), func() {
		from := "start"
		if len(ports) > 0 {
			from = freshIn(route.used, "spread")
			m.w.line("first start then " + from + ";")
			m.w.line("fork " + from + ";")
		}
		m.receptionLoop(r, &route, from, nil)
		for _, p := range ports {
			m.receptionLoop(r, &route, from, p)
		}
	})
	m.w.line("perform action " + writeName(usage) + " : " + writeName(name) + ";")
	m.receptionParameters(r, sig)
	desc := "written as an action def accepting " + m.nameFor(sig)
	if route.performed {
		desc += " and performing its method " + qualifiedName(method)
		if performed != method {
			desc += " as the operation " + qualifiedName(performed) + ", whose body it is"
		}
	}
	m.add(r, verdictFor(route.note), m.v2Name(r), joinNotes(joinNotes(desc+", which its owner performs as "+usage+" from creation, accepting the signal again after each", route.note), info))
}

// receptionRoute is what every accept loop of one reception shares: the signal and the method
// as written (the operation whose body it is), the names taken, and whether it is performed.
type receptionRoute struct {
	owner, sig, method *sysmlv1.Element
	used               map[string]bool
	note               string
	performed          bool
}

// receptionLoop writes one accept loop of reception r reached from node from: an accept of the
// signal from the object, or via port when given, the method performed when it can be, and the return.
func (m *migration) receptionLoop(r *sysmlv1.Element, route *receptionRoute, from string, port *sysmlv1.Element) {
	suffix, via := "", ""
	if port != nil {
		suffix = " via " + m.nameFor(port)
		via = " via " + writeName(m.nameFor(port))
	}
	trig := writeName(freshIn(route.used, "receive"+suffix))
	payload := writeName(freshIn(route.used, lowerFirst(m.nameFor(route.sig))+suffix))
	m.w.line(firstKw + from + thenKw + trig + ";")
	m.w.line(actionKw + trig + " accept " + payload + " : " + m.ref(route.sig, route.owner) + via + ";")
	last := trig
	method := route.method
	switch {
	case method == nil:
		if len(m.model.Unresolved(r, "method")) > 0 {
			route.note = "the method refers to nothing in the document; the reception only accepts the signal"
		} else {
			route.note = "the reception has no method, so it only accepts the signal"
		}
	case !m.written(method) || !(method.Type == "Operation" || hasActionForm(method)):
		route.note = methodNote + qualifiedName(method) + " has no action def to perform; the reception only accepts the signal"
	default:
		args, refusal := m.receptionArguments(method, route.sig, payload)
		if refusal != "" {
			route.note = refusal + "; the reception only accepts the signal"
			break
		}
		run := writeName(freshIn(route.used, "run"+suffix))
		last, route.performed = run, true
		m.w.line(firstKw + trig + thenKw + run + ";")
		decl := actionKw + run + " : " + m.ref(method, route.owner)
		if len(args) == 0 {
			m.w.line(decl + ";")
		} else {
			m.w.line(decl + " { " + strings.Join(args, "; ") + "; }")
		}
	}
	m.w.line(firstKw + last + thenKw + trig + ";")
}

// receptionComment writes a reception whose signal has no v2 declaration as a
// comment, since nothing could accept it.
func (m *migration) receptionComment(r, sig *sysmlv1.Element) {
	text := "reception " + describe(r)
	note := ""
	if sig == nil {
		note = m.dangling(r, "signal")
		if note == "" || len(m.model.Unresolved(r, "signal")) == 0 {
			note = joinNotes(note, "the reception names no signal")
		}
	} else {
		text += " accepts " + qualifiedName(sig)
		note = "the signal " + qualifiedName(sig) + " has no v2 declaration in the document"
	}
	m.w.lines(prefixFirst(commentPrefix, commentLines(text)))
	m.add(r, Unmapped, "", note)
}

// actionParameters lists the parameters the action def written for behavior b declares: an
// operation's own, then its method's that stand for none of them by position or name.
func (m *migration) actionParameters(b *sysmlv1.Element) []*sysmlv1.Element {
	params := b.Owned("ownedParameter")
	method := m.model.Ref(b, "method")
	if b.Type != "Operation" || method == nil || method.Parent != b.Parent {
		return params
	}
	declared := map[string]bool{}
	for _, p := range params {
		declared[m.nameFor(p)] = true
	}
	for _, p := range method.Owned("ownedParameter") {
		if m.realizes[p] != nil || declared[m.nameFor(p)] {
			continue
		}
		declared[m.nameFor(p)] = true
		params = append(params, p)
	}
	return params
}

// receptionArguments binds the method's in parameters to the accepted signal's attributes of the
// same name, conforming in type and multiplicity; one that does not, or is missing where a value
// is required, refuses the method.
func (m *migration) receptionArguments(method, sig *sysmlv1.Element, payload string) (args []string, refusal string) {
	attrs := map[string]*sysmlv1.Element{}
	for _, a := range m.signalAttributes(sig) {
		attrs[m.nameOf(a)] = a
	}
	for _, p := range m.actionParameters(method) {
		dir, _ := parameterDirection(p)
		if dir != "in" && dir != "inout" {
			continue
		}
		name := m.nameOf(p)
		a := attrs[name]
		if name == "" || a == nil {
			if requiresValue(p) {
				refusal = joinNotes(refusal, methodNote+qualifiedName(method)+"'s parameter "+m.nameFor(p)+" must hold a value that no attribute of the signal supplies")
			}
			continue
		}
		if why := m.bindingMismatch(a, p); why != "" {
			refusal = joinNotes(refusal, "the signal's attribute "+name+" "+why+" the method "+qualifiedName(method)+"'s parameter "+m.nameFor(p))
			continue
		}
		args = append(args, m.parameterBinding(p, name, payload+"."+writeName(name)))
	}
	return args, refusal
}

// parameterBinding writes the binding of parameter p, called name, in the body of an action
// usage of its behavior, keeping p's direction so an inout value is written back.
func (m *migration) parameterBinding(p *sysmlv1.Element, name, expr string) string {
	dir, _ := parameterDirection(p)
	return dir + " " + writeName(name) + " = " + expr
}

// bindingMismatch says why feature a cannot be bound to parameter p: its type does not conform
// or its multiplicity does not lie within p's; "" when it can. Non-literal bounds are trusted.
func (m *migration) bindingMismatch(a, p *sysmlv1.Element) string {
	at, pt := m.model.Ref(a, "type"), m.model.Ref(p, "type")
	if !m.conform(at, pt) {
		return "is typed by " + qualifiedName(at) + ", which does not conform to the type " + qualifiedName(pt) + " of"
	}
	al, au, aok := bounds(a)
	pl, pu, pok := bounds(p)
	if aok && pok && (al < pl || (pu >= 0 && (au < 0 || au > pu))) {
		return "has multiplicity " + boundsText(al, au) + ", which does not lie within the " + boundsText(pl, pu) + " of"
	}
	return ""
}

// receptionParameters reports a reception's own parameters, which mirror the
// signal's attributes and are carried by the accepted payload.
func (m *migration) receptionParameters(r, sig *sysmlv1.Element) {
	attrs := map[string]*sysmlv1.Element{}
	for _, a := range m.signalAttributes(sig) {
		attrs[m.nameOf(a)] = a
	}
	for _, p := range r.Owned("ownedParameter") {
		if a := attrs[m.nameOf(p)]; a != nil {
			m.add(p, Mapped, m.v2Name(a), "stands for the signal's attribute "+m.nameOf(a)+", which the accepted payload carries")
			continue
		}
		m.add(p, Unmapped, "", "the parameter "+m.nameFor(p)+" matches no attribute of the signal, whose payload is all the accept carries")
	}
}

// event reports an event declared as a member: it is written as an accept clause
// where a trigger refers to it, and the triggers report those, in their own
// scope; one no trigger refers to is skipped, since no behavior would accept it.
func (m *migration) event(e *sysmlv1.Element) {
	if m.triggered[e] {
		return
	}
	m.add(e, Skipped, "", unreferencedNote+": no trigger refers to the event, so nothing would accept it")
}
