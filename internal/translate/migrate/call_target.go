package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// targetPin settles a call's target pin as an in parameter typed by the class owning op,
// which UML requires its object to be; "" when that class has no v2 type to write.
func (a *activity) targetPin(n, t, op *sysmlv1.Element, used map[string]bool) (pname, typ string) {
	typ, _ = a.m.typeRef(op.Parent, a.def)
	if typ == "" {
		return "", ""
	}
	used[a.m.nameOf(n)] = true
	d := a.settlePin(n, t, "in", used)
	d.typ = op.Parent
	a.m.pins[t] = d
	a.names[t] = d.name
	return d.name, typ
}

// targetNote says what the object a call's target pin holds is, and records the pin.
func (a *activity) targetNote(n, t, op *sysmlv1.Element, pname string) string {
	a.m.add(t, Mapped, a.m.v2Name(n)+"."+pname, "the target pin is the parameter "+pname+", typed by "+qualifiedName(op.Parent)+", the class owning the operation")
	note := "the call runs on the object its target pin " + pname + " holds"
	for _, s := range a.sources[t] {
		if s.Parent != nil && s.Parent.Type == "CreateObjectAction" {
			return note + "; the object is the one " + describe(s.Parent) + " creates, and CreateObjectAction is not migrated, so the pin holds no object and the call acts on an empty pin"
		}
	}
	return note
}

// callOnTarget writes a call of an operation written as a def on the object its target
// pin holds: the pin is a parameter of the usage, which binds the operation's context to it.
func (a *activity) callOnTarget(n, t, op *sysmlv1.Element, name string) (string, bool) {
	c := a.m.contextOf(op)
	used := inheritedActionNamesSet()
	for _, p := range a.m.actionParameters(op) {
		used[a.m.nameFor(p)] = true
	}
	if c != nil {
		used[c.name] = true
	}
	pname, typ := a.targetPin(n, t, op, used)
	if pname == "" {
		return "", false
	}
	ins := ""
	// A def already written, whose body never read its owner, declares no context to bind.
	if c != nil && !(c.owner && c.evaluated && !c.used && !c.bound) {
		ins = "in ref :>> " + writeName(c.name) + " = " + writeName(pname)
		c.bound = true
	}
	members := []string{}
	for _, m := range a.m.contextBody(op, ins) {
		if m != "" {
			members = append(members, m)
		}
	}
	// Last, so that it takes no position of the def's own parameters.
	members = append(members, "in "+writeName(pname)+" : "+typ+"[1]")
	a.m.w.line(actionKw + name + " : " + a.m.ref(op, a.def) + " { " + strings.Join(members, "; ") + "; }")
	return a.targetNote(n, t, op, pname), true
}

// targetBound reports whether the call n is written on the object its target pin t
// holds, t declared as its parameter: a flow feeds t naming no object read from this.
func (a *activity) targetBound(n, t *sysmlv1.Element) bool {
	op := a.m.model.Ref(n, "operation")
	if t == nil || op == nil || op.Parent == nil || a.m.model.Ref(n, "onPort") != nil || len(a.sources[t]) == 0 {
		return false
	}
	if _, _, ok := a.receiverOf(t, op); ok {
		return false
	}
	typ, _ := a.m.typeRef(op.Parent, a.def)
	return typ != ""
}

// callOnTargetLine writes callOnTarget's call, recording it mapped in place of *note; false,
// writing nothing, when the operation's class has no v2 type for the pin.
func (a *activity) callOnTargetLine(n, t, op *sysmlv1.Element, name string, note *string) bool {
	on, ok := a.callOnTarget(n, t, op, name)
	if ok {
		a.m.add(n, Mapped, name, on)
		*note = ""
	}
	return ok
}

// callOnUsageTarget writes a call of an operation written as its block's usage on
// the object its target pin holds, as the mapping's action usage: the pin is its
// parameter, from which a nested perform chains to the operation, and its other
// pins are bound to the operation's parameters by position. It records the call,
// returning the note on any pin left unbound.
func (a *activity) callOnUsageTarget(n, t, op *sysmlv1.Element, name string, ins, outs []*sysmlv1.Element) string {
	used := inheritedActionNamesSet()
	pname, typ := a.targetPin(n, t, op, used)
	usage := a.m.operationUsage(op)
	inParams, outParams := a.m.directedParameters(op)
	var notes []string
	a.m.w.block(actionKw+name, func() {
		a.m.w.line("in " + writeName(pname) + " : " + typ + "[1];")
		a.declarePinsReserving(n, ins, outs, nil, nil, used)
		base := usage
		if used[base] {
			base = usage + " on " + pname
		}
		step := writeName(freshIn(used, base))
		a.m.w.line("perform action " + step + " ::> " + writeName(pname) + "." + writeName(usage) + ";")
		for i, pin := range ins {
			if i < len(inParams) {
				a.m.w.line("bind " + step + "." + writeName(a.m.nameFor(inParams[i])) + " = " + writeName(a.names[pin]) + ";")
			} else {
				notes = append(notes, a.unparametered(pin, "in", op, inParams))
			}
		}
		for i, pin := range outs {
			if i < len(outParams) {
				a.m.w.line("bind " + writeName(a.names[pin]) + " = " + step + "." + writeName(a.m.nameFor(outParams[i])) + ";")
			} else {
				notes = append(notes, a.unparametered(pin, "out", op, outParams))
			}
		}
		a.m.w.line("first start then " + step + ";")
		a.m.w.line("first " + step + " then done;")
	})
	on := a.targetNote(n, t, op, pname) + ", performing the usage " + usage + " of it"
	extra := ""
	for _, n := range notes {
		extra = joinNotes(extra, n)
	}
	a.m.add(n, verdictFor(extra), name, joinNotes(on, extra))
	return extra
}
