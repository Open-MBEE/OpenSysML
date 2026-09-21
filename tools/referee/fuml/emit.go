package fuml

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Package is the package every translated model declares its action definitions in.
const Package = "fuml"

// TranslateError reports a construct of an expressible activity the emitter
// does not spell: which activity, where in it, and why.
type TranslateError struct {
	Activity string
	Where    string
	Reason   string
}

func (e *TranslateError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Activity, e.Where, e.Reason)
}

// Emitted is one activity translated by rule into SysML v2 textual notation,
// with every activity it calls.
type Emitted struct {
	// Name is the model's file name; Text its notation.
	Name string
	Text string
	// Qualified is the qualified name of the action definition the activity became.
	Qualified string
	// Activity is the translated activity.
	Activity *Activity
	// Produces maps each action node of the activity's own flow to the keys
	// (`node.pin`) its output pins have among a run's outputs.
	Produces map[string][]string
	// closure is what the model declares: the classifiers a run's objects and
	// the record's are typed by, each under a name of its own.
	closure *closure
}

// Emit translates an activity and the activities it transitively calls into
// one model. A construct the pilot emitter does not spell is a TranslateError;
// the classifier decides expressibility, the emitter what it can translate.
func Emit(a *Activity) (*Emitted, error) {
	cl, err := closureOf(a)
	if err != nil {
		return nil, err
	}
	spelled, err := spellParameters(a, cl.defs)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, d := range cl.defs {
		if cl.nested(d) {
			continue
		}
		if names[d.Name] {
			return nil, &TranslateError{a.Name, "activity " + d.Name, "two activities in the call closure share the name"}
		}
		names[d.Name] = true
	}
	for _, o := range cl.objects {
		if names[o.name] {
			return nil, &TranslateError{a.Name, "class " + o.name, "shares its name with an activity in the call closure"}
		}
		names[o.name] = true
	}
	for _, sg := range cl.signals {
		if names[sg.Name] {
			return nil, &TranslateError{a.Name, "signal " + sg.Name, "shares its name with an activity or class of the closure"}
		}
		names[sg.Name] = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s {\n\tprivate import ScalarValues::*;\n\tprivate import SequenceFunctions::*;\n\tprivate import ControlFunctions::*;\n", Package)
	em := &Emitted{Name: a.Name + ".sysml", Qualified: Package + "::" + a.Name, Activity: a, closure: cl}
	for _, sg := range cl.signals {
		text, err := cl.emitSignal(sg)
		if err != nil {
			return nil, err
		}
		b.WriteString(text)
	}
	for _, o := range cl.objects {
		text, err := cl.emitObject(o, names, spelled)
		if err != nil {
			return nil, err
		}
		b.WriteString(text)
	}
	for _, d := range cl.defs {
		if cl.nested(d) {
			continue
		}
		text, s, err := cl.emitActivity(d, nil, "\t", names, spelled)
		if err != nil {
			return nil, err
		}
		b.WriteString(text)
		if d == a {
			em.Produces = s.produces()
		}
	}
	b.WriteString("}\n")
	em.Text = b.String()
	return em, nil
}

// closure is what one activity's translation declares: the behaviors whose
// bodies it spells, the classifiers objects are created of, and the signals.
type closure struct {
	root *Activity
	m    *Model
	// defs are the behaviors spelled, in name order: the activity, those it
	// calls, the classifier behaviors it starts, and every activity that is an
	// object's classifier, each spelled once.
	defs []*Activity
	// objects are the part definitions, generals before their specializers.
	objects []*objectDef
	// signals are the attribute definitions, generals before their specializers.
	signals []*Signal
	// instantiated marks each activity whose instances are objects.
	instantiated map[*Activity]bool
	// owner is the part definition each nested behavior is spelled inside.
	owner map[*Activity]*objectDef
}

// objectDef is a classifier objects are created of, spelled as a part
// definition: a class, or an activity instantiated as an object, which runs
// itself when started. A behavior it owns is spelled inside it, so that the
// behavior's `this` is the object performing it.
type objectDef struct {
	name       string
	generals   []TypeRef
	attributes []*Property
	// behaviors are the owned behaviors spelled inside; classifier the one a
	// start runs, bound to the usage named startMember, or nil.
	behaviors  []*Activity
	classifier *Activity
	// class or activity is the classifier spelled.
	class    *Class
	activity *Activity
}

// allAttributes are the attributes objects of the definition hold, inherited ones included.
func (o *objectDef) allAttributes() []*Property {
	if o.class != nil {
		return o.class.AllAttributes()
	}
	return o.activity.Attributes
}

// objectNamed is the part definition the package declares under name, or nil.
func (cl *closure) objectNamed(name string) *objectDef {
	for _, o := range cl.objects {
		if o.name == name {
			return o
		}
	}
	return nil
}

// signalNamed is the attribute definition the package declares under name, or nil.
func (cl *closure) signalNamed(name string) *Signal {
	for _, sg := range cl.signals {
		if sg.Name == name {
			return sg
		}
	}
	return nil
}

// signalOf is the attribute definition signals of the type are created of, or nil.
func (cl *closure) signalOf(t TypeRef) *Signal {
	sg := cl.m.SignalOf(t)
	if sg == nil {
		return nil
	}
	for _, declared := range cl.signals {
		if declared == sg {
			return sg
		}
	}
	return nil
}

// objectOf is the part definition objects of the type are created of, or nil.
func (cl *closure) objectOf(t TypeRef) *objectDef {
	c, act := cl.m.ClassOf(t), cl.m.ActivityOf(t)
	if c == nil && act == nil {
		return nil
	}
	for _, o := range cl.objects {
		if o.class == c && o.activity == act {
			return o
		}
	}
	return nil
}

// startsBehavior reports whether objects of the part definition, or of one it
// specializes, bind a classifier behavior a start runs.
func (cl *closure) startsBehavior(o *objectDef) bool {
	if o.classifier != nil {
		return true
	}
	for _, g := range o.generals {
		if general := cl.objectOf(g); general != nil && cl.startsBehavior(general) {
			return true
		}
	}
	return false
}

// startMember is the usage of a part definition that binds its classifier
// behavior; a start performs `object.classifierBehavior.start`.
const startMember = "classifierBehavior"

// closureOf collects everything the activity's translation declares. An
// activity is an object's classifier when something creates an object of it or
// types an object by it; one the translation also performs as an action, the
// activity itself or one it calls, cannot be both.
func closureOf(a *Activity) (*closure, error) {
	if a.Owner != nil {
		return nil, &TranslateError{a.Name, "activity", "an owned behavior of " + a.Owner.Name +
			" performs only as the behavior of an object of it; on its own it has no object to be this"}
	}
	cl := &closure{root: a, m: a.Model, instantiated: map[*Activity]bool{}, owner: map[*Activity]*objectDef{}}
	if err := cl.behaviors(); err != nil {
		return nil, err
	}
	if err := cl.classifiers(); err != nil {
		return nil, err
	}
	return cl, cl.signalDefs()
}

// nested reports a behavior spelled inside a part definition rather than in the package.
func (cl *closure) nested(d *Activity) bool {
	return cl.owner[d] != nil
}

// behaviors collects defs and instantiated: from the activity, the activities
// it calls, the classifier behaviors of the objects it starts, and every
// activity named as a type, whose body is spelled as its part definition's behavior.
func (cl *closure) behaviors() error {
	seen := map[*Activity]bool{}
	seenClassifiers := map[string]bool{}
	var visit func(d *Activity) error
	var visitType func(t TypeRef) error
	visitType = func(t TypeRef) error {
		var attrs []*Property
		switch c, sg, act := cl.m.ClassOf(t), cl.m.SignalOf(t), cl.m.ActivityOf(t); {
		case c != nil:
			if seenClassifiers[c.ID] {
				return nil
			}
			seenClassifiers[c.ID] = true
			attrs = c.AllAttributes()
		case sg != nil:
			if seenClassifiers[sg.ID] {
				return nil
			}
			seenClassifiers[sg.ID] = true
			attrs = sg.AllAttributes()
		case act != nil:
			if act != cl.root {
				cl.instantiated[act] = true
			}
			return visit(act)
		}
		for _, p := range attrs {
			if err := visitType(p.Type); err != nil {
				return err
			}
		}
		return nil
	}
	visit = func(d *Activity) error {
		if seen[d] {
			return nil
		}
		seen[d] = true
		cl.defs = append(cl.defs, d)
		for _, n := range d.AllNodes() {
			switch n.Kind {
			case CallBehaviorAction:
				if n.Behavior == nil || n.Behavior.Activity == nil {
					continue
				}
				if n.Behavior.Activity.Owner != nil {
					return &TranslateError{cl.root.Name, n.Label(), untranslated("a call of a class's owned behavior")}
				}
				if err := visit(n.Behavior.Activity); err != nil {
					return err
				}
			case StartObjectBehaviorAction:
				for _, p := range n.Inputs() {
					if b := startedBehavior(cl.m, p); p.Role == "object" && b != nil {
						if err := visit(b); err != nil {
							return err
						}
					}
				}
			}
		}
		for _, t := range typeRefs(d) {
			if err := visitType(t); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(cl.root); err != nil {
		return err
	}
	for _, d := range cl.defs {
		if cl.instantiated[d] && (d == cl.root || calls(cl.defs, d)) {
			return &TranslateError{cl.root.Name, "activity " + d.Name, untranslated("an activity both performed as an action and instantiated as an object")}
		}
	}
	sort.Slice(cl.defs, func(i, j int) bool { return cl.defs[i].Name < cl.defs[j].Name })
	return nil
}

// calls reports whether some definition calls the activity.
func calls(defs []*Activity, d *Activity) bool {
	for _, x := range defs {
		for _, n := range x.AllNodes() {
			if n.Kind == CallBehaviorAction && n.Behavior != nil && n.Behavior.Activity == d {
				return true
			}
		}
	}
	return false
}

// typeRefs lists every type an activity names: of its parameters, pins and
// nodes, the classifier it creates, the signals it sends and accepts, and the
// owners of the features it touches.
func typeRefs(d *Activity) []TypeRef {
	var out []TypeRef
	for _, p := range d.Parameters {
		out = append(out, p.Type)
	}
	for _, n := range d.AllNodes() {
		out = append(out, n.Type, n.Classifier, n.Signal)
		for _, tr := range n.Triggers {
			out = append(out, tr.Signal)
		}
		if n.Feature != nil {
			out = append(out, n.Feature.Owner)
		}
	}
	return out
}

// startedBehavior is the behavior a start action's object pin starts: the
// classifier behavior of the pin's type, or of the type flowing into an untyped pin.
func startedBehavior(m *Model, object *Node) *Activity {
	return classifierBehavior(m, orType(object.Type, flowedType(object, map[*Node]bool{})))
}

// classifiers collects objects: every class the definitions name — as a type,
// as the classifier created, or as the owner of a feature touched — with the
// classes those generalize, generals before their specializers, and every
// instantiated activity, each owning its own body.
func (cl *closure) classifiers() error {
	root, m := cl.root, cl.m
	seen := map[*Class]bool{}
	seenSignals := map[*Signal]bool{}
	seenActivities := map[*Activity]bool{}
	var visit func(t TypeRef) error
	visit = func(t TypeRef) error {
		if sg := m.SignalOf(t); sg != nil && !seenSignals[sg] {
			seenSignals[sg] = true
			for _, g := range sg.Generals {
				if err := visit(g); err != nil {
					return err
				}
			}
			for _, p := range sg.Attributes {
				if err := visit(p.Type); err != nil {
					return err
				}
			}
			return nil
		}
		if act := m.ActivityOf(t); act != nil {
			if !cl.instantiated[act] || seenActivities[act] {
				return nil
			}
			seenActivities[act] = true
			for _, p := range act.Attributes {
				if err := visit(p.Type); err != nil {
					return err
				}
			}
			return cl.addObject(&objectDef{name: act.Name, attributes: act.Attributes, behaviors: []*Activity{act}, classifier: act, activity: act})
		}
		c := m.ClassOf(t)
		if c == nil || seen[c] {
			return nil
		}
		seen[c] = true
		for _, g := range c.Generals {
			if m.ClassOf(g) == nil {
				return &TranslateError{root.Name, "class " + c.Name, "generalizes " + g.String() + ", which is no class of the model"}
			}
			if err := visit(g); err != nil {
				return err
			}
		}
		for _, p := range c.Attributes {
			if err := visit(p.Type); err != nil {
				return err
			}
		}
		o := &objectDef{name: c.Name, generals: c.Generals, attributes: c.Attributes, class: c}
		for _, b := range c.Behaviors {
			if cl.spells(b) {
				o.behaviors = append(o.behaviors, b)
			}
		}
		if cl.spells(c.ClassifierBehavior) {
			o.classifier = c.ClassifierBehavior
		}
		return cl.addObject(o)
	}
	for _, d := range cl.defs {
		if d.Owner != nil {
			if err := visit(TypeRef{ID: d.Owner.ID, Name: d.Owner.Name}); err != nil {
				return err
			}
		}
		for _, t := range typeRefs(d) {
			if err := visit(t); err != nil {
				return err
			}
		}
	}
	return nil
}

// spells reports whether the behavior's body is among the definitions.
func (cl *closure) spells(b *Activity) bool {
	for _, d := range cl.defs {
		if d == b {
			return b != nil
		}
	}
	return false
}

// addObject files a part definition, its behaviors nested in it. Its members
// share one namespace: a behavior or the start member named as an attribute
// cannot be told from it.
func (cl *closure) addObject(o *objectDef) error {
	members := map[string]bool{startMember: o.classifier != nil}
	for _, p := range o.attributes {
		if members[p.Name] {
			return &TranslateError{cl.root.Name, "class " + o.name, "has an attribute named " + p.Name + ", as the usage binding its classifier behavior is"}
		}
		members[p.Name] = true
	}
	for _, b := range o.behaviors {
		name := o.behaviorName(b)
		if members[name] {
			return &TranslateError{cl.root.Name, "class " + o.name, "has an attribute named " + name + ", as its owned behavior is"}
		}
		members[name] = true
		for _, p := range b.Parameters {
			if members[p.Name] {
				return &TranslateError{cl.root.Name, "class " + o.name, "has a member named " + p.Name + ", as a parameter of its behavior " + name + " is"}
			}
		}
		cl.owner[b] = o
	}
	cl.objects = append(cl.objects, o)
	return nil
}

// behaviorName is the name a behavior's action definition is declared under
// inside the part definition: its own, or `behavior` for an activity spelled as
// a part definition, whose name the part definition took.
func (o *objectDef) behaviorName(b *Activity) string {
	if b == o.activity {
		return "behavior"
	}
	return b.Name
}

// signalDefs collects signals: every signal the definitions name — as the type
// of a parameter, pin or attribute, as the signal sent, or as an accept's
// trigger — with the signals those generalize, generals before their specializers.
func (cl *closure) signalDefs() error {
	root, m := cl.root, cl.m
	seen := map[*Signal]bool{}
	var visit func(t TypeRef) error
	visit = func(t TypeRef) error {
		sg := m.SignalOf(t)
		if sg == nil || seen[sg] {
			return nil
		}
		seen[sg] = true
		for _, g := range sg.Generals {
			if m.SignalOf(g) == nil {
				return &TranslateError{root.Name, "signal " + sg.Name, "generalizes " + g.String() + ", which is no signal of the model"}
			}
			if err := visit(g); err != nil {
				return err
			}
		}
		for _, p := range sg.Attributes {
			if err := visit(p.Type); err != nil {
				return err
			}
		}
		cl.signals = append(cl.signals, sg)
		return nil
	}
	for _, o := range cl.objects {
		for _, p := range o.attributes {
			if err := visit(p.Type); err != nil {
				return err
			}
		}
	}
	for _, d := range cl.defs {
		for _, p := range d.Parameters {
			if err := visit(p.Type); err != nil {
				return err
			}
		}
		for _, n := range d.AllNodes() {
			if err := visit(n.Type); err != nil {
				return err
			}
			if err := visit(n.Signal); err != nil {
				return err
			}
			for _, tr := range n.Triggers {
				if err := visit(tr.Signal); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// emitSignal spells a signal as an attribute definition: a signal instance is a
// value carried by a message, not an occurrence of its own. Its generals are its
// supertypes, so a specialized signal satisfies an accept of its general; its
// attributes are spelled as a class's are, an object among them referenced.
func (cl *closure) emitSignal(sg *Signal) (string, error) {
	root := cl.root
	var b strings.Builder
	fmt.Fprintf(&b, "\tattribute def %s", quote(sg.Name))
	for i, g := range sg.Generals {
		sep := " :> "
		if i > 0 {
			sep = ", "
		}
		b.WriteString(sep + quote(root.Model.SignalOf(g).Name))
	}
	if len(sg.Attributes) == 0 {
		b.WriteString(";\n")
		return b.String(), nil
	}
	b.WriteString(" {\n")
	for _, p := range sg.Attributes {
		decl, err := cl.attributeDecl(sg.Name, p)
		if err != nil {
			return "", err
		}
		b.WriteString("\t\t" + decl + "\n")
	}
	b.WriteString("\t}\n")
	return b.String(), nil
}

// emitObject spells a classifier as a part definition: an object of it is an
// occurrence with structural features, created and then written to. Its generals
// are its supertypes, its attributes keep their multiplicity exactly. A behavior
// it owns is an action definition nested in it, so `this` in the behavior is the
// object performing it; the classifier behavior is bound by a usage of that
// definition, which a start performs and creation leaves alone.
func (cl *closure) emitObject(o *objectDef, names map[string]bool, spelled map[*Parameter]string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "\tpart def %s", quote(o.name))
	for i, g := range o.generals {
		sep := " :> "
		if i > 0 {
			sep = ", "
		}
		b.WriteString(sep + quote(cl.m.ClassOf(g).Name))
	}
	b.WriteString(" {\n")
	for _, p := range o.attributes {
		decl, err := cl.attributeDecl(o.name, p)
		if err != nil {
			return "", err
		}
		b.WriteString("\t\t" + decl + "\n")
	}
	for _, beh := range o.behaviors {
		text, _, err := cl.emitActivity(beh, o, "\t\t", names, spelled)
		if err != nil {
			return "", err
		}
		b.WriteString(text)
	}
	if o.classifier != nil {
		fmt.Fprintf(&b, "\t\taction %s : %s;\n", startMember, quote(o.behaviorName(o.classifier)))
	}
	b.WriteString("\t}\n")
	return b.String(), nil
}

// attributeDecl spells one attribute of a class or signal: a primitive one an
// attribute, one typed by a class a part (composite) or a reference to one, an
// untyped one an attribute of no type. One redefining an inherited property is
// its redefinition (`:>>`), so it replaces the inherited feature as UML's does.
func (cl *closure) attributeDecl(owner string, p *Property) (string, error) {
	root := cl.root
	where := "attribute " + owner + "." + p.Name
	if p.Association != nil {
		return "", &TranslateError{root.Name, where, untranslated("an association end")}
	}
	name, err := redefinedName(root, where, p)
	if err != nil {
		return "", err
	}
	m := exactMultiplicity(p.Multiplicity)
	switch {
	case p.Type.Zero():
		return fmt.Sprintf("attribute %s%s;", name, m), nil
	case root.Model.primitive(p.Type) != "":
		return fmt.Sprintf("attribute %s : %s%s;", name, root.Model.scalar(root.Model.primitive(p.Type)), m), nil
	case root.Model.SignalOf(p.Type) != nil:
		return fmt.Sprintf("attribute %s : %s%s;", name, quote(root.Model.SignalOf(p.Type).Name), m), nil
	}
	if t := cl.objectOf(p.Type); t != nil {
		kind := "ref part"
		if p.Composite {
			kind = "part"
		}
		return fmt.Sprintf("%s %s : %s%s;", kind, name, quote(t.name), m), nil
	}
	return "", &TranslateError{root.Name, where, untranslated("type " + p.Type.String())}
}

// redefinedName spells a property's declaration name: its own, or `:>> g` for
// one redefining the inherited g of the same name and `n :>> g` for one renaming it.
func redefinedName(root *Activity, where string, p *Property) (string, error) {
	if len(p.Redefines) == 0 {
		return quote(p.Name), nil
	}
	if len(p.Redefines) > 1 {
		return "", &TranslateError{root.Name, where, untranslated("a property redefining several")}
	}
	general := p.Redefines[0]
	if general.Name == p.Name {
		return ":>> " + quote(p.Name), nil
	}
	return quote(p.Name) + " :>> " + quote(general.Name), nil
}

// exactMultiplicity spells a multiplicity as declared, `[1..1]` being the default.
func exactMultiplicity(m Multiplicity) string {
	if m.Lower == 1 && m.Upper == 1 && !m.Ordered && m.Unique {
		return ""
	}
	return " " + m.String()
}

// spellParameters names every parameter: a called activity's is `Activity_name` when
// another definition declares the name, since the runtime returns a nested action's
// outputs to same-named enclosing features; the translated activity's keep theirs.
func spellParameters(root *Activity, defs []*Activity) (map[*Parameter]string, error) {
	declared := map[string]int{}
	for _, d := range defs {
		for _, p := range d.Parameters {
			declared[p.Name]++
		}
	}
	spelled := map[*Parameter]string{}
	for _, d := range defs {
		for _, p := range d.Parameters {
			name := p.Name
			if d != root && declared[name] > 1 {
				name = d.Name + "_" + p.Name
				if declared[name] > 0 {
					return nil, &TranslateError{root.Name, parameterLabel(d.Name + "." + p.Name), "cannot be spelled " + name + ", which another parameter is named"}
				}
			}
			spelled[p] = name
		}
	}
	return spelled, nil
}

// Outputs lists the activity's output, inout and return parameters: the values
// the referee compares.
func (a *Activity) Outputs() []*Parameter {
	var out []*Parameter
	for _, p := range a.Parameters {
		if p.Direction != In {
			out = append(out, p)
		}
	}
	return out
}

// Inputs lists the activity's input and inout parameters.
func (a *Activity) Inputs() []*Parameter {
	var in []*Parameter
	for _, p := range a.Parameters {
		if p.Direction == In || p.Direction == InOut {
			in = append(in, p)
		}
	}
	return in
}

// scalarTypes maps the fUML primitive types to ScalarValues.
var scalarTypes = map[string]string{
	"Integer": "Integer",
	"Boolean": "Boolean",
	"String":  "String",
	"Real":    "Real",
}

// primitive is the ScalarValues type the fUML primitive type t names, or "" when
// t names none: a class, signal or activity of the model named as a primitive is
// that classifier, but no external reference names one, whatever its fragment.
func (m *Model) primitive(t TypeRef) string {
	if m != nil && !t.External &&
		(m.classes[t.ID] != nil || m.signals[t.ID] != nil || m.activities[t.ID] != nil) {
		return ""
	}
	return scalarTypes[t.Name]
}

// scalar spells a ScalarValues type, qualified when a class, signal or activity
// of the model bears its name and would take it over in the package.
func (m *Model) scalar(name string) string {
	if m == nil {
		return name
	}
	for _, c := range m.Classes {
		if c.Name == name {
			return "ScalarValues::" + name
		}
	}
	for _, sg := range m.Signals {
		if sg.Name == name {
			return "ScalarValues::" + name
		}
	}
	for _, a := range m.Activities {
		if a.Name == name {
			return "ScalarValues::" + name
		}
	}
	return name
}

// identRe is a name that needs no quoting.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// quote spells a name in the notation, quoting it unless it is a plain identifier.
func quote(name string) string {
	if identRe.MatchString(name) && !source.IsKeyword(name) {
		return name
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(name) + "'"
}

// emitter translates one activity into one action definition.
type emitter struct {
	a  *Activity
	cl *closure
	// defs names every definition of the package, which no node may shadow.
	defs map[string]bool
	// spelled names every parameter of the model's definitions (spellParameters).
	spelled map[*Parameter]string
}

// scope is one flow: the activity's own or a structured node's. Its nodes are
// declared inside the action, its successions and flows join them.
type scope struct {
	e     *emitter
	owner *Node
	names *namer
	nodes []*snode
	of    map[*Node]*snode
	succs []succession
	flows []flow
	// pins names the feature each fUML pin is spelled as, on its action.
	pins map[*Node]string
	// boundary maps a crossing pin to the feature the structured node declares for it.
	boundary map[*Node]string
	// params are the parameters this scope's action declares, boundary pins included.
	params []string
	// reserved are the names of the enclosing part definition's members.
	reserved []string
}

// snode is one node of the emitted flow.
type snode struct {
	name string
	kind snodeKind
	decl string
	node *Node
	// collector is set for the node that assigns a parameter or boundary pin:
	// it never starts on its own.
	collector bool
	// pending is set for a node that may never fire: an unfed pin with a lower bound.
	pending bool
}

type snodeKind int

const (
	kindAction snodeKind = iota
	kindFork
	kindJoin
	kindMerge
	kindDecide
	kindDone
)

var controlDecl = map[snodeKind]string{kindFork: "fork", kindJoin: "join", kindMerge: "merge", kindDecide: "decide"}

type succession struct {
	src, tgt *snode
	guard    string
}

type flow struct {
	src, tgt       *snode
	srcPin, tgtPin string
}

// namer hands out names unique within a scope.
type namer struct{ used map[string]bool }

func newNamer(reserved ...string) *namer {
	n := &namer{used: map[string]bool{}}
	for _, r := range reserved {
		n.used[r] = true
	}
	return n
}

func (n *namer) name(want string) string {
	name := want
	for i := 2; n.used[name]; i++ {
		name = fmt.Sprintf("%s #%d", want, i)
	}
	n.used[name] = true
	return name
}

// emitActivity spells one activity as an action definition with its parameters:
// in the package, or nested in the part definition owning it. The translated
// activity's own attributes, when objects of it are never created, are those of
// its performance, which `this` names.
func (cl *closure) emitActivity(a *Activity, owner *objectDef, indent string, names map[string]bool, spelled map[*Parameter]string) (string, *scope, error) {
	e := &emitter{a: a, cl: cl, defs: names, spelled: spelled}
	var members []string
	if owner != nil {
		members = append(members, startMember)
		for _, p := range owner.attributes {
			members = append(members, p.Name)
		}
		for _, b := range owner.behaviors {
			members = append(members, owner.behaviorName(b))
		}
	}
	s, err := e.build(nil, a.Nodes, members)
	if err != nil {
		return "", nil, err
	}
	var decls []string
	for _, p := range a.Parameters {
		dir := string(p.Direction)
		if p.Direction == Return {
			dir = "out"
		}
		t, err := e.parameterType(p)
		if err != nil {
			return "", nil, err
		}
		decls = append(decls, parameter(dir, e.spelled[p], t, p.Multiplicity))
	}
	if owner == nil {
		for _, p := range a.Attributes {
			decl, err := cl.attributeDecl(a.Name, p)
			if err != nil {
				return "", nil, err
			}
			decls = append(decls, decl)
		}
	}
	name := a.Name
	if owner != nil {
		name = owner.behaviorName(a)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%saction def %s {\n", indent, quote(name))
	for _, d := range decls {
		b.WriteString(indent + "\t" + d + "\n")
	}
	s.write(&b, indent+"\t")
	b.WriteString(indent + "}\n")
	return b.String(), s, nil
}

// parameter spells a parameter, t being its `: Type` or nothing. An optional
// output holds nothing until a token arrives, so it is declared empty rather
// than left without a value.
func parameter(dir, name, t string, m Multiplicity) string {
	decl := fmt.Sprintf("%s %s%s%s", dir, quote(name), t, multiplicity(m))
	if dir == "out" && m.Lower == 0 {
		decl += " = ()"
	}
	return decl + ";"
}

// produces maps each named action node of the flow to the output keys its pins have.
func (s *scope) produces() map[string][]string {
	out := map[string][]string{}
	for _, n := range s.nodes {
		if n.node == nil || n.kind != kindAction || n.node.Name == "" || !n.node.Kind.Action() {
			continue
		}
		for _, pin := range n.node.Outputs() {
			if f := s.pins[pin]; f != "" {
				out[n.node.Name] = append(out[n.node.Name], n.name+"."+f)
			}
		}
	}
	return out
}

// multiplicity spells a multiplicity. A multi-valued one is `[0..*] nonunique`:
// it fills one token at a time and the implementation keeps every token whatever
// isUnique says; ordered is kept, as the referee compares an ordered parameter in order.
func multiplicity(m Multiplicity) string {
	switch {
	case m.Lower == 1 && m.Upper == 1:
		return ""
	case m.Upper == 1:
		return fmt.Sprintf("[%d..1]", m.Lower)
	case m.Ordered:
		return "[0..*] ordered nonunique"
	}
	return "[0..*] nonunique"
}

// parameterType spells a parameter's `: Type`; an untyped parameter stays
// untyped, as the implementation runs it, and is spelled as nothing.
func (e *emitter) parameterType(p *Parameter) (string, error) {
	if p.Type.Zero() {
		return "", nil
	}
	t, err := e.typeOf(p.Type, parameterLabel(p.Name))
	if err != nil {
		return "", err
	}
	return " : " + t, nil
}

// typeOf spells a primitive type, a classifier objects are created of by its
// part definition, a signal by its attribute definition, or the translated
// activity itself, the type of its performance, by its action definition.
func (e *emitter) typeOf(t TypeRef, where string) (string, error) {
	if t.Zero() {
		return "", &TranslateError{e.a.Name, where, "has no type"}
	}
	if name := e.a.Model.primitive(t); name != "" {
		return e.a.Model.scalar(name), nil
	}
	if o := e.cl.objectOf(t); o != nil {
		return quote(o.name), nil
	}
	if sg := e.a.Model.SignalOf(t); sg != nil {
		return quote(sg.Name), nil
	}
	if e.performance(t) {
		return quote(e.a.Name), nil
	}
	return "", &TranslateError{e.a.Name, where, untranslated("type " + t.String())}
}

// performance reports whether the type is the translated activity's own, whose
// instance is the performance `this` names when no object is created of it.
func (e *emitter) performance(t TypeRef) bool {
	return e.a == e.cl.root && e.a.Model.ActivityOf(t) == e.a && !e.cl.nested(e.a)
}

func (e *emitter) fail(where, reason string) error {
	return &TranslateError{e.a.Name, where, reason}
}

// untranslated is the reason for a construct the pilot emitter has no spelling for.
func untranslated(what string) string {
	return what + " is not translated by the pilot emitter"
}

// parameterLabel names a parameter as the where of a translation error.
func parameterLabel(name string) string {
	return "parameter " + name
}

// undeclaredNode is the reason for an edge whose end the emitter never declared.
const undeclaredNode = "joins a node the emitter did not declare"

// pinType spells a pin's type. A library list function leaves its element type
// open, so an untyped input pin takes the type of the value flowing into it and an
// untyped result pin the element type of the call's inputs.
func (e *emitter) pinType(p *Node) (string, error) {
	if !p.Type.Zero() {
		return e.typeOf(p.Type, pinLabel(p))
	}
	if p.Kind == InputPin {
		if t := flowedType(p, map[*Node]bool{}); !t.Zero() {
			return e.typeOf(t, pinLabel(p))
		}
	} else if p.Owner != nil {
		for _, in := range p.Owner.Inputs() {
			if t, err := e.pinType(in); err == nil {
				return t, nil
			}
		}
	}
	return "", e.fail(pinLabel(p), "has no type")
}

// flowedType is the type of the value reaching an untyped node along its
// incoming object flows, looked for back through control nodes.
func flowedType(n *Node, seen map[*Node]bool) TypeRef {
	if seen[n] {
		return TypeRef{}
	}
	seen[n] = true
	for _, in := range n.Incoming {
		if in.Kind != ObjectFlow {
			continue
		}
		if t := effectiveType(in.Source); !t.Zero() {
			return t
		}
		if t := flowedType(in.Source, seen); !t.Zero() {
			return t
		}
	}
	return TypeRef{}
}

// build translates the nodes of one flow and the edges among them; no node
// takes a reserved name, a definition's, a parameter's or a member's of the
// part definition enclosing the flow.
func (e *emitter) build(owner *Node, nodes []*Node, reserved []string) (*scope, error) {
	s := &scope{
		e: e, owner: owner,
		names:    newNamer("start", "done", "Start"),
		of:       map[*Node]*snode{},
		pins:     map[*Node]string{},
		boundary: map[*Node]string{},
		reserved: reserved,
	}
	for name := range e.defs {
		s.names.used[name] = true
	}
	for _, name := range reserved {
		s.names.used[name] = true
	}
	for _, p := range e.a.Parameters {
		s.names.used[e.spelled[p]] = true
	}
	for _, n := range nodes {
		if err := s.declare(n); err != nil {
			return nil, err
		}
	}
	for _, edge := range e.a.Edges {
		if err := s.edge(edge); err != nil {
			return nil, err
		}
	}
	if err := s.trace(); err != nil {
		return nil, err
	}
	s.start()
	s.fanOut()
	return s, nil
}

// scopeOf is the structured node whose flow the node is in, nil for the
// activity's own flow; a pin is in its action's.
func scopeOf(n *Node) *Node {
	o := n.Owner
	if n.Kind.Pin() && o != nil {
		o = o.Owner
	}
	return o
}

// declare spells one node.
func (s *scope) declare(n *Node) error {
	e := s.e
	switch n.Kind {
	case FlowFinalNode:
		return nil
	case ActivityFinalNode:
		s.add(&snode{name: "done", kind: kindDone, node: n})
		return nil
	case InitialNode, ForkNode:
		s.control(n, kindFork)
	case JoinNode:
		s.control(n, kindJoin)
	case MergeNode:
		s.control(n, kindMerge)
	case DecisionNode:
		s.control(n, kindDecide)
	case ActivityParameterNode:
		return s.parameterNode(n)
	case ValueSpecificationAction:
		return s.valueNode(n)
	case CallBehaviorAction:
		return s.callNode(n)
	case StructuredActivityNode:
		return s.structuredNode(n)
	case CreateObjectAction:
		return s.createNode(n)
	case ReadSelfAction:
		return s.selfNode(n)
	case ReadStructuralFeatureAction, AddStructuralFeatureValueAction, RemoveStructuralFeatureValueAction, ClearStructuralFeatureAction:
		return s.featureNode(n)
	case SendSignalAction:
		return s.sendNode(n)
	case AcceptEventAction:
		return s.acceptNode(n)
	case StartObjectBehaviorAction:
		return s.startNode(n)
	default:
		return e.fail(n.Label(), untranslated(string(n.Kind)))
	}
	return nil
}

func (s *scope) add(n *snode) *snode {
	s.nodes = append(s.nodes, n)
	if n.node != nil {
		s.of[n.node] = n
	}
	return n
}

func (s *scope) control(n *Node, kind snodeKind) {
	name := s.names.name(nodeName(n))
	s.add(&snode{name: name, kind: kind, node: n, decl: controlDecl[kind] + " " + quote(name) + ";"})
}

// nodeName is the name a node is declared under before uniquing.
func nodeName(n *Node) string {
	if n.Name != "" {
		return n.Name
	}
	return fmt.Sprintf("%s(%d)", n.Kind, n.Line)
}

// parameterNode spells an activity parameter node: an input's reads the
// parameter into a pin, an output's collects what arrives into the parameter.
// An inout parameter has a node of each kind, told apart by the edges it has.
func (s *scope) parameterNode(n *Node) error {
	p := n.Parameter
	if p == nil {
		return s.e.fail(n.Label(), "names no parameter")
	}
	if s.owner != nil {
		return s.e.fail(n.Label(), untranslated("an activity parameter node inside a structured node"))
	}
	t, err := s.e.parameterType(p)
	if err != nil {
		return err
	}
	name := s.names.name(nodeName(n))
	s.pins[n] = "v"
	feature := quote(s.e.spelled[p])
	if p.Direction == In || p.Direction == InOut && len(n.Incoming) == 0 {
		s.add(&snode{name: name, kind: kindAction, node: n,
			decl: fmt.Sprintf("action %s { out v%s%s = %s; }", quote(name), t, multiplicity(p.Multiplicity), feature)})
		return nil
	}
	s.add(&snode{name: name, kind: kindAction, node: n, collector: true,
		decl: fmt.Sprintf("action %s { %s }", quote(name), collect(feature, t, p.Multiplicity))})
	return nil
}

// collect spells the body of an action that takes v, of `: Type` t or untyped,
// and puts it into target: a scalar is assigned, a multi-valued target appended to.
func collect(target, t string, m Multiplicity) string {
	if m.Upper == 1 {
		return fmt.Sprintf("in v%s[0..1]; assign %s := v;", t, target)
	}
	return fmt.Sprintf("in v%s[0..*] nonunique; assign %s := (%s, v);", t, target, target)
}

// valueNode spells a value specification action: an action with one output pin
// holding the literal.
func (s *scope) valueNode(n *Node) error {
	e := s.e
	outs := n.Outputs()
	if len(outs) != 1 || len(n.Inputs()) != 0 {
		return e.fail(n.Label(), "a value specification action has one result pin")
	}
	if n.Value == nil {
		return e.fail(n.Label(), "has no value")
	}
	// The implementation puts the evaluated literal on the pin whatever the pin's
	// type says, so the literal's kind types the pin.
	pin := outs[0]
	t, ok := literalTypes[n.Value.Kind]
	if !ok {
		return e.fail(n.Label(), untranslated(n.Value.Kind))
	}
	lit, err := e.literal(n.Value, t, n.Label())
	if err != nil {
		return err
	}
	name := s.names.name(nodeName(n))
	pinName := "result"
	s.pins[pin] = pinName
	s.add(&snode{name: name, kind: kindAction, node: n,
		decl: fmt.Sprintf("action %s { out %s : %s%s = %s; }", quote(name), pinName, e.a.Model.scalar(t), multiplicity(pin.Multiplicity), lit)})
	return nil
}

// literalTypes is the primitive type each literal kind evaluates to. An unlimited
// natural the classifier admits only positions a structural feature value, and a
// position is spelled as an Integer, `*` as unlimited.
var literalTypes = map[string]string{
	"LiteralInteger":          "Integer",
	"LiteralBoolean":          "Boolean",
	"LiteralString":           "String",
	"LiteralReal":             "Real",
	"LiteralUnlimitedNatural": "Integer",
}

// unlimited is the Integer an unlimited natural `*` is spelled as: no position is negative.
const unlimited = "-1"

// literal spells a literal value of the given primitive type.
func (e *emitter) literal(v *Value, t, where string) (string, error) {
	text := v.Text
	switch v.Kind {
	case "LiteralInteger":
		if !v.Given {
			text = "0"
		}
		if _, err := strconv.ParseInt(text, 10, 64); err != nil || t != "Integer" {
			return "", e.fail(where, fmt.Sprintf("LiteralInteger %q does not spell an %s", text, t))
		}
		return text, nil
	case "LiteralBoolean":
		if !v.Given {
			text = "false"
		}
		if (text != "true" && text != "false") || t != "Boolean" {
			return "", e.fail(where, fmt.Sprintf("LiteralBoolean %q does not spell a %s", text, t))
		}
		return text, nil
	case "LiteralString":
		if t != "String" {
			return "", e.fail(where, "LiteralString does not spell a "+t)
		}
		return strconv.Quote(text), nil
	case "LiteralReal":
		if !v.Given {
			text = "0"
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || t != "Real" {
			return "", e.fail(where, fmt.Sprintf("LiteralReal %q does not spell a %s", text, t))
		}
		return strconv.FormatFloat(f, 'f', -1, 64) + realSuffix(f), nil
	case "LiteralUnlimitedNatural":
		if !v.Given {
			text = "0"
		}
		if text == "*" {
			text = unlimited
		}
		if n, err := strconv.ParseInt(text, 10, 64); err != nil || n < -1 || t != "Integer" {
			return "", e.fail(where, fmt.Sprintf("LiteralUnlimitedNatural %q does not spell a position", v.Text))
		}
		return text, nil
	}
	return "", e.fail(where, untranslated(v.Kind))
}

// createNode spells a create object action: an action whose result pin holds a
// new occurrence of the classifier's part definition. Creating starts no behavior.
func (s *scope) createNode(n *Node) error {
	e := s.e
	outs := n.Outputs()
	if len(outs) != 1 || len(n.Inputs()) != 0 {
		return e.fail(n.Label(), "a create object action has one result pin")
	}
	o := e.cl.objectOf(n.Classifier)
	if o == nil {
		return e.fail(n.Label(), "creates a "+n.Classifier.String()+", which is no class of the model")
	}
	name := s.names.name(nodeName(n))
	s.pins[outs[0]] = "result"
	s.add(&snode{name: name, kind: kindAction, node: n,
		decl: fmt.Sprintf("action %s { out result : %s = new %s(); }", quote(name), quote(o.name), quote(o.name))})
	return nil
}

// selfNode spells a read self action as `this`, the object whose behavior the
// activity is spelled inside; performed on its own, self is no object.
func (s *scope) selfNode(n *Node) error {
	e := s.e
	outs := n.Outputs()
	if len(outs) != 1 || len(n.Inputs()) != 0 {
		return e.fail(n.Label(), "a read self action has one result pin")
	}
	o := e.cl.owner[e.a]
	if o == nil {
		return e.fail(n.Label(), "reads self in an activity performed on its own, where self is the performance and not an object")
	}
	name := s.names.name(nodeName(n))
	s.pins[outs[0]] = "result"
	s.add(&snode{name: name, kind: kindAction, node: n,
		decl: fmt.Sprintf("action %s { out result : %s = this; }", quote(name), quote(o.name))})
	return nil
}

// startNode spells a start object behavior action as a performance of the
// object's classifier behavior: `perform object.classifierBehavior.start`, which
// runs the behavior asynchronously on the object, as fUML's start does. The
// object's type binds the behavior, own or inherited; an object without one, or
// a start passing arguments to the behavior, is refused.
func (s *scope) startNode(n *Node) error {
	e := s.e
	if len(n.Outputs()) != 0 {
		return e.fail(n.Label(), "a start object behavior action has no result pin")
	}
	var object *Node
	for _, p := range n.Inputs() {
		switch p.Role {
		case "object":
			if object != nil {
				return e.fail(n.Label(), "has two object pins")
			}
			object = p
		case "argument":
			return e.fail(n.Label(), untranslated("a start passing arguments to the behavior"))
		default:
			return e.fail(n.Label(), "has a "+p.Role+" pin")
		}
	}
	if object == nil {
		return e.fail(n.Label(), "has no object pin")
	}
	t := orType(object.Type, flowedType(object, map[*Node]bool{}))
	o := e.cl.objectOf(t)
	if o == nil {
		return e.fail(n.Label(), "starts a "+t.String()+", which is no class of the model")
	}
	if !e.cl.startsBehavior(o) {
		return e.fail(n.Label(), "starts an object of "+o.name+", which has no classifier behavior")
	}
	s.pins[object] = "object"
	name := s.names.name(nodeName(n))
	s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
		decl: fmt.Sprintf("action %s { in object : %s; perform object.%s.start; }", quote(name), quote(o.name), startMember)})
	return nil
}

// featureNode spells a structural feature action: an action taking the object
// (and the value, and the position) at its pins, whose body reads the object's
// feature onto its result pin or assigns the feature and hands the object on.
func (s *scope) featureNode(n *Node) error {
	e := s.e
	f := n.Feature
	if f == nil {
		return e.fail(n.Label(), "names no structural feature")
	}
	if f.Association != nil {
		return e.fail(n.Label(), untranslated("an association end"))
	}
	if e.cl.objectOf(f.Owner) == nil && !e.performance(f.Owner) {
		return e.fail(n.Label(), "touches a feature of "+f.Owner.String()+", which is no class of the model")
	}
	pins := map[string]*Node{}
	for _, p := range n.Pins {
		if pins[p.Role] != nil {
			return e.fail(n.Label(), "has two "+p.Role+" pins")
		}
		pins[p.Role] = p
	}
	object := pins["object"]
	if object == nil {
		return e.fail(n.Label(), "has no object pin")
	}
	objectType, err := e.typeOf(orType(object.Type, f.Owner), pinLabel(object))
	if err != nil {
		return err
	}
	s.pins[object] = "object"
	features := []string{fmt.Sprintf("in object : %s;", objectType)}
	value := pins["value"]
	writes := n.Kind == AddStructuralFeatureValueAction || n.Kind == RemoveStructuralFeatureValueAction
	switch {
	case value != nil && !writes:
		return e.fail(n.Label(), "takes a value it has no use for")
	case value == nil && writes:
		return e.fail(n.Label(), "has no value pin")
	case value != nil:
		t, err := e.pinType(value)
		if err != nil {
			return err
		}
		s.pins[value] = "value"
		features = append(features, fmt.Sprintf("in value : %s;", t))
	}
	position := pins["insertAt"]
	if n.Kind == RemoveStructuralFeatureValueAction {
		position = pins["removeAt"]
	}
	if position != nil {
		s.pins[position] = position.Role
		features = append(features, fmt.Sprintf("in %s : %s;", position.Role, e.a.Model.scalar("Integer")))
	}
	result := pins["result"]
	if n.Kind == ReadStructuralFeatureAction {
		if result == nil {
			return e.fail(n.Label(), "has no result pin")
		}
		s.pins[result] = "result"
		m := multiplicity(Multiplicity{Upper: f.Upper, Ordered: f.Ordered, Unique: f.Unique})
		t := ""
		if rt := orType(result.Type, f.Type); !rt.Zero() {
			if t, err = e.typeOf(rt, pinLabel(result)); err != nil {
				return err
			}
			t = " : " + t
		}
		features = append(features, fmt.Sprintf("out result%s%s = object.%s;", t, m, quote(f.Name)))
	} else {
		if result != nil {
			s.pins[result] = "result"
			features = append(features, fmt.Sprintf("out result : %s = object;", objectType))
		}
		features = append(features, fmt.Sprintf("assign object.%s := %s;", quote(f.Name), featureUpdate(n, f, position != nil)))
	}
	name := s.names.name(nodeName(n))
	s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
		decl: fmt.Sprintf("action %s { %s }", quote(name), strings.Join(features, " "))})
	return nil
}

// sendNode spells a send signal action: an action taking the target object and
// the signal's attribute values at its pins, whose body sends a new instance of
// the signal to the target. The send completes without waiting, as fUML's does.
func (s *scope) sendNode(n *Node) error {
	e := s.e
	sg := e.a.Model.SignalOf(n.Signal)
	if sg == nil {
		return e.fail(n.Label(), "sends "+n.Signal.String()+", which is no signal of the model")
	}
	if len(n.Outputs()) != 0 {
		return e.fail(n.Label(), "a send signal action has no result pin")
	}
	var target *Node
	var args []*Node
	for _, p := range n.Inputs() {
		switch p.Role {
		case "target":
			if target != nil {
				return e.fail(n.Label(), "has two target pins")
			}
			target = p
		case "argument":
			args = append(args, p)
		default:
			return e.fail(n.Label(), "has a "+p.Role+" pin")
		}
	}
	if target == nil {
		return e.fail(n.Label(), "has no target pin")
	}
	attrs := sg.AllAttributes()
	if len(args) != len(attrs) {
		return e.fail(n.Label(), fmt.Sprintf("%s has %d attributes, the send has %d argument pins", sg.Name, len(attrs), len(args)))
	}
	targetType, err := e.pinType(target)
	if err != nil {
		return err
	}
	pins := newNamer("target")
	s.pins[target] = "target"
	features := []string{fmt.Sprintf("in target : %s;", targetType)}
	var values []string
	for i, p := range args {
		t, err := e.pinType(p)
		if err != nil {
			return err
		}
		pn := pins.name(pinFeature(p, attrs[i].Name))
		s.pins[p] = pn
		features = append(features, fmt.Sprintf("in %s : %s%s;", quote(pn), t, multiplicity(p.Multiplicity)))
		values = append(values, quote(attrs[i].Name)+" = "+quote(pn))
	}
	features = append(features, fmt.Sprintf("send new %s(%s) to target;", quote(sg.Name), strings.Join(values, ", ")))
	name := s.names.name(nodeName(n))
	s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
		decl: fmt.Sprintf("action %s { %s }", quote(name), strings.Join(features, " "))})
	return nil
}

// acceptNode spells an accept event action as an accept node waiting for the
// signal its trigger names, the accepted instance being its result pin. An
// instance of a specialized signal satisfies an accept of its general, as in fUML.
func (s *scope) acceptNode(n *Node) error {
	e := s.e
	if n.Unmarshall {
		return e.fail(n.Label(), "unmarshalls the signal onto one pin per attribute, which the pilot emitter does not spell")
	}
	if len(n.Triggers) != 1 {
		return e.fail(n.Label(), fmt.Sprintf("has %d triggers; a SysML v2 accept names one signal", len(n.Triggers)))
	}
	tr := n.Triggers[0]
	if tr.Operation != nil {
		return e.fail(n.Label(), "accepts a call event, which SysML v2 has no counterpart for")
	}
	sg := e.a.Model.SignalOf(tr.Signal)
	if sg == nil {
		return e.fail(n.Label(), "accepts "+tr.Signal.String()+", which is no signal of the model")
	}
	if len(n.Inputs()) != 0 {
		return e.fail(n.Label(), "an accept event action has no input pin")
	}
	outs := n.Outputs()
	if len(outs) > 1 {
		return e.fail(n.Label(), "an accept event action of one signal has one result pin")
	}
	name := s.names.name(nodeName(n))
	if len(outs) == 0 {
		s.add(&snode{name: name, kind: kindAction, node: n, decl: fmt.Sprintf("action %s accept %s;", quote(name), quote(sg.Name))})
		return nil
	}
	// The runtime binds the payload in the enclosing flow under its name as well as
	// on the node, so the name is kept apart from every other of the scope.
	payload := s.names.name(pinFeature(outs[0], "result"))
	s.pins[outs[0]] = payload
	s.add(&snode{name: name, kind: kindAction, node: n,
		decl: fmt.Sprintf("action %s accept %s : %s;", quote(name), quote(payload), quote(sg.Name))})
	return nil
}

// orType is t, or fallback when t names no type.
func orType(t, fallback TypeRef) TypeRef {
	if t.Zero() {
		return fallback
	}
	return t
}

// featureUpdate spells what a feature holds after the action, as the reference
// implementation computes it. Add: a replacing add or a single-valued feature
// takes the value outright; otherwise a unique feature drops its old copy, and the
// value goes at the one-based position given (the runtime rejecting any other),
// `*` appending and none inserting first.
// Remove: every copy (removeDuplicates), the value at the position given, or the
// first copy; a single-valued feature is emptied when its one value is the one
// positioned or held. Clear empties.
func featureUpdate(n *Node, f *Property, positioned bool) string {
	held := "object." + quote(f.Name)
	switch {
	case n.Kind == ClearStructuralFeatureAction:
		return "()"
	case n.Kind == AddStructuralFeatureValueAction && (n.ReplaceAll || f.Upper == 1):
		return "value"
	case n.Kind == AddStructuralFeatureValueAction:
		base := held
		if f.Unique {
			base = "excluding(" + held + ", value)"
		}
		if !positioned {
			return "(value, " + base + ")"
		}
		return fmt.Sprintf("if insertAt < 0 ? including(%s, value) else includingAt(%s, value, insertAt)", base, base)
	case f.Upper == 1 && positioned && !n.RemoveDuplicates:
		return fmt.Sprintf("if removeAt == 1 ? () else %s", held)
	case f.Upper == 1:
		return fmt.Sprintf("if %s == value ? () else %s", held, held)
	case n.RemoveDuplicates:
		return "excluding(" + held + ", value)"
	case positioned:
		return fmt.Sprintf("if removeAt >= 1 and removeAt <= size(%s) ? excludingAt(%s, removeAt) else %s", held, held, held)
	}
	first := fmt.Sprintf("(1..size(%s))->select { in k; %s#(k) == value }#(1)", held, held)
	return fmt.Sprintf("if includes(%s, value) ? excludingAt(%s, %s) else %s", held, held, first, held)
}

// realSuffix makes a real literal spell as one: an integral value gets `.0`.
func realSuffix(f float64) string {
	if f == float64(int64(f)) {
		return ".0"
	}
	return ""
}

// callNode spells a call behavior action: of an activity, an action typed by its
// definition, whose pins are the definition's parameters; of a library function,
// an action computing its result from its argument pins.
func (s *scope) callNode(n *Node) error {
	e := s.e
	b := n.Behavior
	if b == nil {
		return e.fail(n.Label(), "names no behavior")
	}
	name := s.names.name(nodeName(n))
	if b.Activity != nil {
		if err := s.bindCallPins(n, b.Activity); err != nil {
			return err
		}
		s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
			decl: fmt.Sprintf("action %s : %s::%s;", quote(name), Package, quote(b.Activity.Name))})
		return nil
	}
	template, ok := libraryCalls[b.Name]
	if !ok {
		return e.fail(n.Label(), "calls "+b.Name+", which the pilot emitter does not spell")
	}
	pins := newNamer()
	var features, args []string
	for i, p := range n.Inputs() {
		t, err := e.pinType(p)
		if err != nil {
			return err
		}
		pn := pins.name(pinFeature(p, fmt.Sprintf("argument%d", i)))
		s.pins[p] = pn
		args = append(args, quote(pn))
		features = append(features, fmt.Sprintf("in %s : %s%s;", quote(pn), t, multiplicity(p.Multiplicity)))
	}
	outs := n.Outputs()
	if len(outs) != 1 {
		return e.fail(n.Label(), "a library function call has one result pin")
	}
	if len(args) != template.arity {
		return e.fail(n.Label(), fmt.Sprintf("%s takes %d arguments, the call has %d pins", b.Name, template.arity, len(args)))
	}
	r := outs[0]
	t, err := e.pinType(r)
	if err != nil {
		return err
	}
	rn := pins.name(pinFeature(r, "result"))
	s.pins[r] = rn
	features = append(features, fmt.Sprintf("out %s : %s%s = %s;", quote(rn), t, multiplicity(r.Multiplicity), template.spell(args)))
	s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
		decl: fmt.Sprintf("action %s { %s }", quote(name), strings.Join(features, " "))})
	return nil
}

// pinFeature is the feature a pin of a generated action is spelled as.
func pinFeature(p *Node, fallback string) string {
	if p.Name != "" {
		return p.Name
	}
	return fallback
}

// unfed reports an action with an input pin that must hold a value and no edge
// to bring one: the implementation never fires it.
func unfed(n *Node) bool {
	for _, p := range n.Inputs() {
		if p.Multiplicity.Lower > 0 && len(p.Incoming) == 0 {
			return true
		}
	}
	return false
}

// bindCallPins names each pin of a call of an activity after the parameter it
// stands for: the argument pins are the in and inout parameters in order, the
// result pins the out, inout and return parameters.
func (s *scope) bindCallPins(n *Node, callee *Activity) error {
	e := s.e
	var ins, outs []*Parameter
	for _, p := range callee.Parameters {
		if p.Direction == In || p.Direction == InOut {
			ins = append(ins, p)
		}
		if p.Direction != In {
			outs = append(outs, p)
		}
	}
	for i, p := range n.Inputs() {
		if i >= len(ins) {
			return e.fail(pinLabel(p), fmt.Sprintf("%s has %d input parameters, the call %d argument pins", callee.Name, len(ins), len(n.Inputs())))
		}
		s.pins[p] = e.spelled[ins[i]]
	}
	for i, p := range n.Outputs() {
		if i >= len(outs) {
			return e.fail(pinLabel(p), fmt.Sprintf("%s has %d output parameters, the call %d result pins", callee.Name, len(outs), len(n.Outputs())))
		}
		s.pins[p] = e.spelled[outs[i]]
	}
	return nil
}

// structuredNode spells a structured activity node as a nested action owning
// the flow of its contents. A pin inside it that an edge crosses the boundary
// to reach becomes a parameter of the nested action.
func (s *scope) structuredNode(n *Node) error {
	e := s.e
	if len(n.Pins) > 0 {
		return e.fail(n.Label(), untranslated("a structured node with pins"))
	}
	inner, err := e.build(n, n.Nodes, s.reserved)
	if err != nil {
		return err
	}
	name := s.names.name(nodeName(n))
	var b strings.Builder
	fmt.Fprintf(&b, "action %s {\n", quote(name))
	for _, p := range inner.params {
		b.WriteString("\t" + p + "\n")
	}
	inner.write(&b, "\t")
	b.WriteString("}")
	sn := s.add(&snode{name: name, kind: kindAction, node: n, decl: b.String()})
	// The boundary features the inner scope declared are this node's pins here.
	for pin, feature := range inner.boundary {
		s.pins[pin] = feature
		s.of[pin] = sn
	}
	return nil
}

// endpoint is the node an edge end is spelled at: a pin's action, a node's own.
func (s *scope) endpoint(n *Node) *snode {
	if n.Kind.Pin() {
		if sn, ok := s.of[n]; ok {
			return sn
		}
		return s.of[n.Owner]
	}
	return s.of[n]
}

// edge translates one edge whose ends are in this scope, or that crosses its
// boundary from the enclosing scope.
func (s *scope) edge(edge *Edge) error {
	e := s.e
	src, tgt := edge.Source, edge.Target
	if src == nil || tgt == nil {
		return e.fail("edge at line "+strconv.Itoa(edge.Line), "has an unresolved end")
	}
	from, to := scopeOf(src), scopeOf(tgt)
	if from != s.owner && to != s.owner {
		return nil
	}
	if from != to {
		return s.crossing(edge, from, to)
	}
	if !unitWeight(edge) {
		return e.fail(edgeLabel(edge), untranslated("an edge weight other than 1"))
	}
	if src.Kind == FlowFinalNode || src.Kind == ActivityFinalNode {
		return e.fail(edgeLabel(edge), "a final node has no outgoing edge")
	}
	if tgt.Kind == FlowFinalNode {
		return nil
	}
	a, b := s.endpoint(src), s.endpoint(tgt)
	if a == nil || b == nil {
		return e.fail(edgeLabel(edge), undeclaredNode)
	}
	guard := ""
	if edge.Guard != nil {
		if a.kind != kindDecide {
			return e.fail(edgeLabel(edge), "a guard is spelled on a decision's outgoing edge only")
		}
		expr, err := s.guard(src, edge)
		if err != nil {
			return err
		}
		guard = expr
	} else if a.kind == kindDecide {
		return e.fail(edgeLabel(edge), "a decision's outgoing edge needs a guard")
	}
	if edge.Kind == ObjectFlow && a.kind == kindAction && b.kind == kindAction {
		// The object flow itself is spelled by trace; here it enables its target.
		s.enable(a, b)
		return nil
	}
	s.succs = append(s.succs, succession{a, b, guard})
	return nil
}

// unitWeight reports an edge carrying one token per firing, UML's default.
func unitWeight(e *Edge) bool {
	w := e.Weight
	return w == nil || (w.Kind == "LiteralInteger" || w.Kind == "LiteralUnlimitedNatural") && w.Given && w.Text == "1"
}

// enable adds the succession an object flow implies, once per pair of actions;
// a collector performs once per flow, since each performance takes one delivery.
func (s *scope) enable(a, b *snode) {
	for _, x := range s.succs {
		if x.src == a && x.tgt == b && !b.collector {
			return
		}
	}
	s.succs = append(s.succs, succession{src: a, tgt: b})
}

// crossing translates an object flow between this scope and a structured node
// directly inside it: the node gets a parameter the flow ends at, and its inner
// scope a node reading or collecting it.
func (s *scope) crossing(edge *Edge, from, to *Node) error {
	e := s.e
	if edge.Kind != ObjectFlow {
		return e.fail(edgeLabel(edge), untranslated("a control flow across a structured node's boundary"))
	}
	if edge.Guard != nil || !unitWeight(edge) {
		return e.fail(edgeLabel(edge), untranslated("a guarded or weighted flow across a structured node's boundary"))
	}
	src, tgt := edge.Source, edge.Target
	switch {
	case s.owner == from && to != nil && to.Owner == s.owner && tgt.Kind.Pin():
		// Entering the structured node: the outer half, ending at its new parameter.
		sn, a := s.of[to], s.endpoint(src)
		if sn == nil || a == nil {
			return e.fail(edgeLabel(edge), undeclaredNode)
		}
		if _, ok := s.pins[tgt]; !ok {
			return e.fail(edgeLabel(edge), "enters a structured node at a pin its inner flow did not declare")
		}
		if s.pins[src] == "" {
			return e.fail(edgeLabel(edge), "enters a structured node from a node without a pin, which the pilot emitter does not translate")
		}
		s.flows = append(s.flows, flow{a, sn, s.pins[src], s.pins[tgt]})
		s.enable(a, sn)
	case s.owner == to && from != nil && from.Owner == s.owner && src.Kind.Pin():
		// Leaving the structured node: the outer half, starting at its new parameter.
		sn, b := s.of[from], s.endpoint(tgt)
		if sn == nil || b == nil {
			return e.fail(edgeLabel(edge), undeclaredNode)
		}
		if _, ok := s.pins[src]; !ok {
			return e.fail(edgeLabel(edge), "leaves a structured node at a pin its inner flow did not declare")
		}
		if b.kind != kindAction || s.pins[tgt] == "" {
			return e.fail(edgeLabel(edge), untranslated("a flow leaving a structured node into a control node"))
		}
		s.flows = append(s.flows, flow{sn, b, s.pins[src], s.pins[tgt]})
		s.enable(sn, b)
	case s.owner == to && s.owner != nil && from == s.owner.Owner && tgt.Kind.Pin():
		// Entering: the inner half, a node reading the parameter into the pin.
		return s.boundaryIn(edge, tgt)
	case s.owner == from && s.owner != nil && to == s.owner.Owner && src.Kind.Pin():
		// Leaving: the inner half, a node collecting the pin into the parameter.
		return s.boundaryOut(edge, src)
	default:
		return e.fail(edgeLabel(edge), "crosses more than one structured node boundary, which the pilot emitter does not translate")
	}
	return nil
}

// boundaryIn declares the parameter an entering flow delivers to and the node
// that reads it to the pin it was headed for.
func (s *scope) boundaryIn(edge *Edge, pin *Node) error {
	t, err := s.e.typeOf(pin.Type, pinLabel(pin))
	if err != nil {
		return err
	}
	feature, ok := s.boundary[pin]
	if !ok {
		feature = s.names.name("in(" + pinLabel(pin) + ")")
		s.boundary[pin] = feature
		s.params = append(s.params, parameter("in", feature, " : "+t, pin.Multiplicity))
	}
	name := s.names.name("read(" + pinLabel(pin) + ")")
	reader := s.add(&snode{name: name, kind: kindAction,
		decl: fmt.Sprintf("action %s { out v : %s%s = %s; }", quote(name), t, multiplicity(pin.Multiplicity), quote(feature))})
	tgt := s.endpoint(pin)
	if tgt == nil {
		return s.e.fail(edgeLabel(edge), "enters a node the emitter did not declare")
	}
	s.flows = append(s.flows, flow{reader, tgt, "v", s.pins[pin]})
	s.enable(reader, tgt)
	return nil
}

// boundaryOut declares the parameter a leaving flow carries out and the node
// that collects the pin into it.
func (s *scope) boundaryOut(edge *Edge, pin *Node) error {
	t, err := s.e.typeOf(pin.Type, pinLabel(pin))
	if err != nil {
		return err
	}
	feature, ok := s.boundary[pin]
	if !ok {
		feature = s.names.name("out(" + pinLabel(pin) + ")")
		s.boundary[pin] = feature
		s.params = append(s.params, parameter("out", feature, " : "+t, pin.Multiplicity))
	}
	name := s.names.name("write(" + pinLabel(pin) + ")")
	writer := s.add(&snode{name: name, kind: kindAction, collector: true,
		decl: fmt.Sprintf("action %s { %s }", quote(name), collect(quote(feature), " : "+t, pin.Multiplicity))})
	src := s.endpoint(pin)
	if src == nil {
		return s.e.fail(edgeLabel(edge), "leaves a node the emitter did not declare")
	}
	s.flows = append(s.flows, flow{src, writer, s.pins[pin], "v"})
	s.enable(src, writer)
	return nil
}

func edgeLabel(e *Edge) string {
	if e.Name != "" {
		return string(e.Kind) + " " + e.Name
	}
	return fmt.Sprintf("%s %s -> %s", e.Kind, e.Source.Label(), e.Target.Label())
}

// guard spells the guard of a decision's outgoing edge as a test of the value
// the decision decides on.
func (s *scope) guard(decision *Node, edge *Edge) (string, error) {
	e := s.e
	input, err := s.decisionInput(decision)
	if err != nil {
		return "", err
	}
	typ := effectiveType(input)
	spelled, err := e.typeOf(typ, pinLabel(input))
	if err != nil {
		return "", err
	}
	t := e.a.Model.primitive(typ)
	if t == "" {
		return "", e.fail(edgeLabel(edge), "guards a "+spelled+", which no literal spells")
	}
	lit, err := e.literal(edge.Guard, t, edgeLabel(edge))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s == %s", quote(s.endpoint(input).name), quote(s.pins[input]), lit), nil
}

// decisionInput is the pin whose value a decision decides on: its decision
// input flow's source, else the source of its one incoming object flow, looked
// for back through control nodes with a single input.
func (s *scope) decisionInput(d *Node) (*Node, error) {
	e := s.e
	var edge *Edge
	if d.DecisionInputFlow != nil {
		edge = d.DecisionInputFlow
	} else {
		for _, in := range d.Incoming {
			if in.Kind != ObjectFlow {
				continue
			}
			if edge != nil {
				return nil, e.fail(d.Label(), "decides on more than one incoming object flow")
			}
			edge = in
		}
	}
	if edge == nil {
		return nil, e.fail(d.Label(), "decides on a control token, which has no value to guard on")
	}
	src := edge.Source
	for src != nil && src.Kind.Control() {
		var in *Edge
		for _, x := range src.Incoming {
			if x.Kind != ObjectFlow {
				continue
			}
			if in != nil {
				return nil, e.fail(d.Label(), "decides on a value merged from more than one source")
			}
			in = x
		}
		if in == nil {
			return nil, e.fail(d.Label(), "decides on a control token, which has no value to guard on")
		}
		src = in.Source
	}
	if src == nil || s.endpoint(src) == nil || s.pins[src] == "" {
		return nil, e.fail(d.Label(), "decides on a value the emitter did not spell")
	}
	return src, nil
}

// trace spells every object flow end to end: from each pin or parameter node
// that produces a value, through the control nodes routing its token, to every
// pin or parameter node the token can reach. Each route is one flow.
func (s *scope) trace() error {
	for _, n := range s.nodes {
		if n.node == nil || n.kind != kindAction {
			continue
		}
		var sources []*Node
		if n.node.Kind == ActivityParameterNode {
			sources = []*Node{n.node}
		} else {
			sources = n.node.Outputs()
		}
		for _, src := range sources {
			if s.pins[src] == "" {
				continue
			}
			for _, out := range src.Outgoing {
				if out.Kind != ObjectFlow || scopeOf(out.Target) != s.owner {
					continue
				}
				if err := s.route(n, src, out, map[*Edge]bool{}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// route follows one object token from its source pin along an edge.
func (s *scope) route(src *snode, srcPin *Node, edge *Edge, seen map[*Edge]bool) error {
	e := s.e
	if seen[edge] {
		return e.fail(edgeLabel(edge), untranslated("an object flow cycle through control nodes"))
	}
	seen[edge] = true
	defer delete(seen, edge)
	tgt := edge.Target
	switch {
	case tgt.Kind == FlowFinalNode || tgt.Kind == ActivityFinalNode:
		return nil
	case tgt.Kind.Control():
		for _, out := range tgt.Outgoing {
			if out.Kind != ObjectFlow {
				return e.fail(edgeLabel(out), "a control flow carries an object token on")
			}
			if err := s.route(src, srcPin, out, seen); err != nil {
				return err
			}
		}
		return nil
	case tgt.Kind.Pin() || tgt.Kind == ActivityParameterNode:
		to := s.endpoint(tgt)
		if to == nil || s.pins[tgt] == "" {
			return e.fail(edgeLabel(edge), "delivers to a pin the emitter did not spell")
		}
		s.flows = append(s.flows, flow{src, to, s.pins[srcPin], s.pins[tgt]})
		return nil
	}
	return e.fail(edgeLabel(edge), "delivers an object token to a "+string(tgt.Kind))
}

// start enables the nodes the implementation enables when the flow begins: those
// with nothing coming in. One is enabled by start itself, several through a fork.
func (s *scope) start() {
	incoming := map[*snode]int{}
	for _, x := range s.succs {
		incoming[x.tgt]++
	}
	var starts []*snode
	for _, n := range s.nodes {
		if n.kind == kindDone || n.collector || n.pending || incoming[n] > 0 {
			continue
		}
		starts = append(starts, n)
	}
	if len(starts) == 0 {
		return
	}
	from := &snode{name: "start"}
	if len(starts) > 1 {
		from = &snode{name: "Start", kind: kindFork, decl: "fork Start;"}
		s.nodes = append([]*snode{from}, s.nodes...)
		s.succs = append(s.succs, succession{src: &snode{name: "start"}, tgt: from})
	}
	for _, n := range starts {
		s.succs = append(s.succs, succession{src: from, tgt: n})
	}
}

// fanOut spells the implicit fork of an action with several successors, and the
// implicit merge of several edges into the final node or into a collector: a
// parameter node takes tokens as they come, so its collector performs per arrival.
func (s *scope) fanOut() {
	outgoing := map[*snode][]int{}
	incomingMerged := map[*snode][]int{}
	for i, x := range s.succs {
		outgoing[x.src] = append(outgoing[x.src], i)
		if x.tgt.kind == kindDone || x.tgt.collector {
			incomingMerged[x.tgt] = append(incomingMerged[x.tgt], i)
		}
	}
	for _, n := range append([]*snode(nil), s.nodes...) {
		if idx := outgoing[n]; len(idx) > 1 && n.kind != kindFork && n.kind != kindDecide {
			name := s.names.name(n.name + " fork")
			f := s.add(&snode{name: name, kind: kindFork, decl: "fork " + quote(name) + ";"})
			for _, i := range idx {
				s.succs[i].src = f
			}
			s.succs = append(s.succs, succession{src: n, tgt: f})
		}
		if idx := incomingMerged[n]; len(idx) > 1 {
			name := s.names.name(n.name + " merge")
			m := s.add(&snode{name: name, kind: kindMerge, decl: "merge " + quote(name) + ";"})
			for _, i := range idx {
				s.succs[i].tgt = m
			}
			s.succs = append(s.succs, succession{src: m, tgt: n})
		}
	}
}

// write spells the scope's declarations, then its flows and successions.
func (s *scope) write(b *strings.Builder, indent string) {
	for _, n := range s.nodes {
		if n.decl == "" {
			continue
		}
		for _, line := range strings.Split(n.decl, "\n") {
			b.WriteString(indent + line + "\n")
		}
	}
	for _, f := range s.flows {
		fmt.Fprintf(b, "%sflow %s.%s to %s.%s;\n", indent, quote(f.src.name), quote(f.srcPin), quote(f.tgt.name), quote(f.tgtPin))
	}
	for _, x := range s.succs {
		if x.guard != "" {
			fmt.Fprintf(b, "%ssuccession first %s if %s then %s;\n", indent, quote(x.src.name), x.guard, quote(x.tgt.name))
			continue
		}
		fmt.Fprintf(b, "%ssuccession first %s then %s;\n", indent, quote(x.src.name), quote(x.tgt.name))
	}
}

// libraryCall spells a call of a fUML library function over its arguments.
type libraryCall struct {
	arity int
	spell func(args []string) string
}

// call spells a KerML function applied to every argument in order.
func call(fn string, arity int) libraryCall {
	return libraryCall{arity, func(args []string) string { return fn + "(" + strings.Join(args, ", ") + ")" }}
}

// libraryCalls spells each library behavior LibraryCounterpart names.
var libraryCalls = map[string]libraryCall{
	"PrimitiveBehaviors::IntegerFunctions::+":   call("IntegerFunctions::'+'", 2),
	"PrimitiveBehaviors::IntegerFunctions::-":   call("IntegerFunctions::'-'", 2),
	"PrimitiveBehaviors::IntegerFunctions::*":   call("IntegerFunctions::'*'", 2),
	"PrimitiveBehaviors::IntegerFunctions::Neg": call("IntegerFunctions::'-'", 1),
	"PrimitiveBehaviors::IntegerFunctions::Div": {2, func(a []string) string {
		return fmt.Sprintf("RationalFunctions::ToInteger(IntegerFunctions::'/'(%s, %s))", a[0], a[1])
	}},
	"PrimitiveBehaviors::IntegerFunctions::Mod":      call("IntegerFunctions::'%'", 2),
	"PrimitiveBehaviors::IntegerFunctions::Abs":      call("IntegerFunctions::abs", 1),
	"PrimitiveBehaviors::IntegerFunctions::Max":      call("IntegerFunctions::max", 2),
	"PrimitiveBehaviors::IntegerFunctions::Min":      call("IntegerFunctions::min", 2),
	"PrimitiveBehaviors::IntegerFunctions::<":        call("IntegerFunctions::'<'", 2),
	"PrimitiveBehaviors::IntegerFunctions::<=":       call("IntegerFunctions::'<='", 2),
	"PrimitiveBehaviors::IntegerFunctions::>=":       call("IntegerFunctions::'>='", 2),
	"PrimitiveBehaviors::RealFunctions::+":           call("RealFunctions::'+'", 2),
	"PrimitiveBehaviors::RealFunctions::-":           call("RealFunctions::'-'", 2),
	"PrimitiveBehaviors::RealFunctions::*":           call("RealFunctions::'*'", 2),
	"PrimitiveBehaviors::RealFunctions::/":           call("RealFunctions::'/'", 2),
	"PrimitiveBehaviors::RealFunctions::Neg":         call("RealFunctions::'-'", 1),
	"PrimitiveBehaviors::RealFunctions::Inv":         {1, func(a []string) string { return "RealFunctions::'/'(1.0, " + a[0] + ")" }},
	"PrimitiveBehaviors::RealFunctions::Abs":         call("RealFunctions::abs", 1),
	"PrimitiveBehaviors::RealFunctions::Floor":       call("RealFunctions::floor", 1),
	"PrimitiveBehaviors::RealFunctions::Round":       call("RealFunctions::round", 1),
	"PrimitiveBehaviors::RealFunctions::Max":         call("RealFunctions::max", 2),
	"PrimitiveBehaviors::RealFunctions::Min":         call("RealFunctions::min", 2),
	"PrimitiveBehaviors::RealFunctions::<":           call("RealFunctions::'<'", 2),
	"PrimitiveBehaviors::RealFunctions::<=":          call("RealFunctions::'<='", 2),
	"PrimitiveBehaviors::RealFunctions::>":           call("RealFunctions::'>'", 2),
	"PrimitiveBehaviors::RealFunctions::>=":          call("RealFunctions::'>='", 2),
	"PrimitiveBehaviors::RealFunctions::ToInteger":   call("RealFunctions::ToInteger", 1),
	"PrimitiveBehaviors::BooleanFunctions::And":      call("BooleanFunctions::'&'", 2),
	"PrimitiveBehaviors::BooleanFunctions::Or":       call("BooleanFunctions::'|'", 2),
	"PrimitiveBehaviors::BooleanFunctions::Not":      call("BooleanFunctions::'not'", 1),
	"PrimitiveBehaviors::BooleanFunctions::Xor":      call("BooleanFunctions::'xor'", 2),
	"PrimitiveBehaviors::BooleanFunctions::Implies":  call("ControlFunctions::'implies'", 2),
	"PrimitiveBehaviors::StringFunctions::Concat":    call("StringFunctions::'+'", 2),
	"PrimitiveBehaviors::StringFunctions::Size":      call("StringFunctions::Length", 1),
	"PrimitiveBehaviors::StringFunctions::Substring": call("StringFunctions::Substring", 3),
	"PrimitiveBehaviors::ListFunctions::ListSize":    call("SequenceFunctions::size", 1),
	"PrimitiveBehaviors::ListFunctions::ListGet":     call("SequenceFunctions::'#'", 2),
	"PrimitiveBehaviors::ListFunctions::ListConcat":  call("SequenceFunctions::union", 2),
}

// IsTranslateError reports whether err is the emitter declining a construct.
func IsTranslateError(err error) bool {
	var t *TranslateError
	return errors.As(err, &t)
}
