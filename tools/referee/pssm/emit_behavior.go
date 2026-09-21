package pssm

import (
	"fmt"
	"strings"
)

// binding is the binding of a behavior with parameters, or the binder's
// reason none holds; a behavior without parameters has none and no error.
func (e *emitter) binding(bh *Behavior, where string) (*Binding, error) {
	if !hasParams(bh) {
		return nil, nil
	}
	if r := e.bindings.refusal(bh); r != nil {
		return nil, e.fail(where, r.Reason)
	}
	binding := e.bindings.Bound[bh]
	if binding == nil {
		return nil, e.fail(where, "a behavior with parameters the binder did not visit")
	}
	for _, p := range bh.Params {
		if p.Name == "log" {
			return nil, e.fail(where, "a parameter named log shadows the machine's log")
		}
	}
	return binding, nil
}

// bindValues spells what each input reads: the accept's own parameters in the
// accepting transition's effect, elsewhere the attributes it stored them in.
func (e *emitter) bindValues(binding *Binding) []string {
	values := make([]string, len(binding.Data))
	for i, p := range binding.Data {
		switch {
		case binding.Direct == nil:
			values[i] = e.bindings.carriedAttr(binding.Event, p)
		case binding.Event.Kind == EventSignal:
			values[i] = spell(payloadParam(binding.Event.Signal.Name))
		default:
			values[i] = spell(p.Name)
		}
	}
	return values
}

// carryEventData declares an attribute per value of each event's data that a
// bound behavior reads after the accepting transition's effect stored it.
func (e *emitter) carryEventData() error {
	declared := map[string]bool{}
	for _, ev := range e.bindings.carriedEvents() {
		data, _ := eventData(ev)
		for _, p := range data {
			attr := e.bindings.carriedAttr(ev, p)
			if e.targetAttribute(attr) {
				return e.fail("attribute "+attr, "the target declares the attribute the translation would carry the event's data in")
			}
			if declared[attr] {
				return e.fail("attribute "+attr, "two carried values spell the same attribute name")
			}
			declared[attr] = true
			decl := fmt.Sprintf("attribute %s : %s", attr, e.dataType(ev, p))
			if ev.Kind == EventCall {
				decl += " = " + zeroLiteral(scalarTypes[p.Type])
			}
			e.carriedAttrs = append(e.carriedAttrs, decl+";")
		}
	}
	return nil
}

// targetAttribute reports whether the target class declares an attribute.
func (e *emitter) targetAttribute(name string) bool {
	if e.test.Target == nil {
		return false
	}
	for _, a := range e.test.Target.Attributes {
		if a.Name == name {
			return true
		}
	}
	return false
}

// storeEventData spells the assignments opening a triggered transition's
// effect, storing each value of the accepted event's data in its attribute.
func (e *emitter) storeEventData(ev *Event, param string) []string {
	data, _ := eventData(ev)
	var out []string
	for _, p := range data {
		value := spell(p.Name)
		if ev.Kind == EventSignal {
			value = spell(param)
		}
		out = append(out, fmt.Sprintf("assign %s := %s;", e.bindings.carriedAttr(ev, p), value))
	}
	return out
}

// dataType spells the type of one value of an event's data: the signal itself
// for a payload, the ScalarValues type for a call's input.
func (e *emitter) dataType(ev *Event, p Param) string {
	if ev.Kind == EventSignal {
		e.signals[ev.Signal.Name] = true
		return ev.Signal.Name
	}
	return scalarTypes[p.Type]
}

// boundEntry spells a bound entry as the text after `entry action`: a usage of
// its definition when it has outputs, otherwise inputs then statements inline.
func (e *emitter) boundEntry(bh *Behavior, ind, where, base string) (string, error) {
	binding, err := e.binding(bh, where)
	if err != nil {
		return "", err
	}
	if len(outputs(bh)) > 0 {
		return e.defUsage(binding, ind, where, base)
	}
	params, scope, types := e.declaredInputs(binding)
	stmts, err := scoped(e, scope, types, func() ([]string, error) {
		return e.plainBody(bh, where)
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(" {\n")
	writeStmts(&b, ind+"    ", params)
	writeStmts(&b, ind+"    ", stmts)
	fmt.Fprintf(&b, "%s}", ind)
	return b.String(), nil
}

// boundEffect spells a bound effect as the transition's `do` statements: a usage
// of its definition when it has outputs, otherwise the statements inline.
func (e *emitter) boundEffect(bh *Behavior, ind, where, base string) ([]string, error) {
	binding, err := e.binding(bh, where)
	if err != nil {
		return nil, err
	}
	if len(outputs(bh)) > 0 {
		usage, err := e.defUsage(binding, ind, where, base)
		if err != nil {
			return nil, err
		}
		return []string{"action" + usage}, nil
	}
	scope, types := map[string]string{}, map[string]string{}
	for i, p := range inputs(bh) {
		scope[p.Name] = e.bindValues(binding)[i]
		types[p.Name] = p.Type
	}
	return scoped(e, scope, types, func() ([]string, error) {
		return e.plainBody(bh, where)
	})
}

// declaredInputs spells a bound behavior's inputs as parameters of the action
// usage, `in p : T = <value>;`, with the scope that names them.
func (e *emitter) declaredInputs(binding *Binding) (params []string, scope, types map[string]string) {
	scope, types = map[string]string{}, map[string]string{}
	values := e.bindValues(binding)
	for i, p := range inputs(binding.Behavior) {
		scope[p.Name] = spell(p.Name)
		types[p.Name] = p.Type
		params = append(params, fmt.Sprintf("in %s : %s = %s;", spell(p.Name), e.dataType(binding.Event, binding.Data[i]), values[i]))
	}
	return params, scope, types
}

// defUsage spells ` : <def> { inout log = log; in p = <value>; ... }`, inputs in
// the definition's order since a usage's parameters redefine by position.
func (e *emitter) defUsage(binding *Binding, ind, where, base string) (string, error) {
	name, err := e.definition(binding, where, base)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, " : %s {\n%s    inout log = log;\n", name, ind)
	for i, p := range inputs(binding.Behavior) {
		dir, name := inputSpelling(binding, p)
		fmt.Fprintf(&b, "%s    %s %s = %s;\n", ind, dir, name, e.bindValues(binding)[i])
	}
	fmt.Fprintf(&b, "%s}", ind)
	return b.String(), nil
}

// inputSpelling is the direction and feature name an input is declared under:
// an inout is one feature, named as it returns.
func inputSpelling(binding *Binding, p Param) (dir, name string) {
	if p.Direction != "inout" {
		return "in", spell(p.Name)
	}
	for i, o := range outputs(binding.Behavior) {
		if o.Name == p.Name {
			return "inout", spell(binding.Outputs[i])
		}
	}
	return "inout", spell(p.Name)
}

// inoutHolders names, per inout parameter the body writes, the attribute holding
// the value until the body ends: UML posts it when the activity completes.
func inoutHolders(bh *Behavior, scope map[string]string) map[string]string {
	taken := map[string]bool{"log": true}
	for _, name := range scope {
		taken[name] = true
	}
	holders := map[string]string{}
	for _, p := range bh.Params {
		if p.Direction != "inout" || !writes(bh.Body, p.Name) {
			continue
		}
		name := scope[p.Name] + "_written"
		for n := 2; taken[name]; n++ {
			name = fmt.Sprintf("%s_written_%d", scope[p.Name], n)
		}
		taken[name] = true
		holders[p.Name] = name
	}
	return holders
}

// writes reports whether a body returns through the named parameter.
func writes(body *Body, param string) bool {
	for _, st := range body.Statements {
		if st.Kind == StmtReturn && st.Feature == param {
			return true
		}
	}
	return false
}

// definition spells a bound behavior as an action definition once: `inout log`
// first (a definition reaches no feature of the machine), inputs, then outputs.
func (e *emitter) definition(binding *Binding, where, base string) (string, error) {
	bh := binding.Behavior
	if name, ok := e.defNames[bh]; ok {
		return name, nil
	}
	if bh.Body == nil {
		return "", e.fail(where, "an opaque behavior has no translation")
	}
	for _, name := range binding.Outputs {
		if e.targetAttribute(name) {
			return "", e.fail(where, fmt.Sprintf("output %s would return through the target's attribute of that name", name))
		}
	}
	name := e.defName(base)
	e.defNames[bh] = name
	scope := map[string]string{}
	types := map[string]string{}
	var b strings.Builder
	fmt.Fprintf(&b, "    action def %s {\n        inout log : String;\n", name)
	for i, p := range inputs(bh) {
		dir, feat := inputSpelling(binding, p)
		scope[p.Name] = feat
		types[p.Name] = p.Type
		fmt.Fprintf(&b, "        %s %s : %s;\n", dir, feat, e.dataType(binding.Event, binding.Data[i]))
	}
	for i, p := range outputs(bh) {
		if p.Direction == "inout" {
			continue
		}
		scope[p.Name] = spell(binding.Outputs[i])
		types[p.Name] = p.Type
		fmt.Fprintf(&b, "        out %s : %s;\n", spell(binding.Outputs[i]), scalarTypes[p.Type])
	}
	holders := inoutHolders(bh, scope)
	stmts, err := scoped(e, scope, types, func() ([]string, error) {
		e.scopeWrites = holders
		return e.plainBody(bh, where)
	})
	if err != nil {
		return "", err
	}
	for _, p := range bh.Params {
		if holder, ok := holders[p.Name]; ok {
			fmt.Fprintf(&b, "        attribute %s : %s = %s;\n", holder, scalarTypes[p.Type], zeroLiteral(scalarTypes[p.Type]))
			stmts = append(stmts, fmt.Sprintf("assign %s := %s;", scope[p.Name], holder))
		}
	}
	b.WriteString("        first start;\n")
	if len(stmts) > 0 {
		b.WriteString("        then action body {\n")
		writeStmts(&b, "            ", stmts)
		b.WriteString("        }\n")
	}
	b.WriteString("        then done;\n    }\n")
	e.defs = append(e.defs, b.String())
	return name, nil
}

// defName derives a definition's name from the behavior's site, unique within
// the model.
func (e *emitter) defName(site string) string {
	base := identifier(site)
	taken := map[string]bool{}
	for _, n := range e.defNames {
		taken[n] = true
	}
	name := base
	for n := 2; taken[name]; n++ {
		name = fmt.Sprintf("%s_%d", base, n)
	}
	return name
}

// scoped spells with the parameters of one behavior in scope, restoring the
// enclosing scope after.
func scoped[T any](e *emitter, scope, types map[string]string, spell func() (T, error)) (T, error) {
	outerScope, outerTypes, outerWrites := e.scope, e.scopeTypes, e.scopeWrites
	e.scope, e.scopeTypes, e.scopeWrites = scope, types, nil
	defer func() { e.scope, e.scopeTypes, e.scopeWrites = outerScope, outerTypes, outerWrites }()
	return spell()
}

// stepReturn translates a bound behavior's return as an assignment to the
// output the caller receives, or to the attribute holding an inout's value
// until the body ends; outside a definition it has no destination.
func (e *emitter) stepReturn(out *stepList, st Statement, where string, depth int) error {
	target, ok := e.scope[st.Feature]
	if !ok {
		return e.fail(where, "a return has no translation")
	}
	if holder, ok := e.scopeWrites[st.Feature]; ok {
		target = holder
	}
	value, err := e.expr(out, st.Value, where, depth)
	if err != nil {
		return err
	}
	out.add(fmt.Sprintf("assign %s := %s;", target, value))
	return nil
}

// libraryForms spells the fUML primitives as the KerML operator or function of
// the same meaning (BaseFunctions::ToString prints as fUML's ToString does).
var libraryForms = map[string]struct {
	arity int
	form  string
}{
	"IntegerFunctions::plus":              {2, "(%s + %s)"},
	"IntegerFunctions::minus":             {2, "(%s - %s)"},
	"IntegerFunctions::times":             {2, "(%s * %s)"},
	"IntegerFunctions::lt":                {2, "(%s < %s)"},
	"IntegerFunctions::le":                {2, "(%s <= %s)"},
	"IntegerFunctions::gt":                {2, "(%s > %s)"},
	"IntegerFunctions::ge":                {2, "(%s >= %s)"},
	"IntegerFunctions::ToString":          {1, "ToString(%s)"},
	"UnlimitedNaturalFunctions::ToString": {1, "ToString(%s)"},
	"BooleanFunctions::Not":               {1, "(not %s)"},
	"BooleanFunctions::And":               {2, "(%s and %s)"},
	"BooleanFunctions::Or":                {2, "(%s or %s)"},
	"BooleanFunctions::Xor":               {2, "(%s xor %s)"},
	"BooleanFunctions::ToString":          {1, "ToString(%s)"},
	"StringFunctions::Concat":             {2, "(%s + %s)"},
}

// formatParameterValue is the suite's Util::Tracing::formatParameterValue.
const formatParameterValueQualified = "Util::Tracing::formatParameterValue"

// apply spells an application: an owned behavior is inlined, a library one is
// its expression, and formatParameterValue brackets as the suite's library does.
func (e *emitter) apply(out *stepList, x *Expr, where string, depth int) (string, error) {
	if x.Library == nil {
		if bh := e.ownedBehavior(x.BehaviorID); bh != nil {
			return e.inline(out, bh, x, where, depth)
		}
	}
	args := make([]string, len(x.Args))
	for i := range x.Args {
		a, err := e.expr(out, &x.Args[i], where, depth)
		if err != nil {
			return "", err
		}
		args[i] = a
	}
	switch {
	case x.Library == nil && x.Name == "==" && len(args) == 2:
		return fmt.Sprintf("(%s == %s)", args[0], args[1]), nil
	case x.Library == nil:
		return "", e.fail(where, fmt.Sprintf("%s applies a behavior the target does not own", x))
	case x.Library.Qualified == formatParameterValueQualified && len(args) == 2:
		prefix := fmt.Sprintf(`(if %s ? "[in=" else "[out=")`, args[0])
		if lit := x.Args[0].Literal; x.Args[0].Kind == ExprLiteral && lit != nil && lit.Kind == LiteralBoolean {
			prefix = `"[in="`
			if lit.String() != "true" {
				prefix = `"[out="`
			}
		}
		return fmt.Sprintf(`(%s + ToString(%s) + "]")`, prefix, args[1]), nil
	}
	form, ok := libraryForms[x.Library.Qualified]
	if !ok || form.arity != len(args) {
		return "", e.fail(where, fmt.Sprintf("%s applies a behavior the translation does not spell", x))
	}
	if form.arity == 1 {
		return fmt.Sprintf(form.form, args[0]), nil
	}
	return fmt.Sprintf(form.form, args[0], args[1]), nil
}

// ownedBehavior finds the behavior the target class owns by xmi:id.
func (e *emitter) ownedBehavior(id string) *Behavior {
	if e.test.Target == nil || id == "" {
		return nil
	}
	for _, bh := range e.test.Target.Behaviors {
		if bh.ID == id {
			return bh
		}
	}
	return nil
}

// inline spells an owned behavior's application: its statements run where it is
// evaluated, parameters standing for the arguments, its one return the value.
func (e *emitter) inline(out *stepList, bh *Behavior, x *Expr, where string, depth int) (string, error) {
	if depth > 8 {
		return "", e.fail(where, "behaviors apply each other too deeply to inline")
	}
	if bh.Body == nil {
		return "", e.fail(where, fmt.Sprintf("%s applies an opaque behavior", x))
	}
	ins, outs := inputs(bh), outputs(bh)
	if len(ins) != len(x.Args) {
		return "", e.fail(where, fmt.Sprintf("%s passes %d arguments to %d parameters", x, len(x.Args), len(ins)))
	}
	if len(outs) != 1 {
		return "", e.fail(where, fmt.Sprintf("%s applies a behavior with %d outputs, not a value", x, len(outs)))
	}
	scope := map[string]string{}
	types := map[string]string{}
	for i, p := range ins {
		a, err := e.expr(out, &x.Args[i], where, depth)
		if err != nil {
			return "", err
		}
		scope[p.Name] = a
		types[p.Name] = p.Type
	}
	inner := where + " > " + bh.Name
	value := ""
	_, err := scoped(e, scope, types, func() ([]string, error) {
		if len(bh.Body.Unsupported) > 0 {
			return nil, e.fail(inner, "activity nodes with no translation: "+strings.Join(bh.Body.Unsupported, "; "))
		}
		for _, st := range bh.Body.Statements {
			if st.Kind == StmtReturn && st.Feature == outs[0].Name {
				if value != "" {
					return nil, e.fail(inner, "returns twice")
				}
				v, err := e.expr(out, st.Value, inner, depth+1)
				if err != nil {
					return nil, err
				}
				value = v
				continue
			}
			if err := e.stmt(out, st, inner, depth+1); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", e.fail(inner, "returns no value")
	}
	return value, nil
}
