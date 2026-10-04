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
	used := map[string]bool{}
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
