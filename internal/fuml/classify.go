package fuml

import (
	"fmt"
	"sort"
	"strings"
)

// Bucket is the verdict the referee files an activity under.
type Bucket string

// The four buckets, in report order. fUML has no `terminate`, so there is no
// terminate-gap bucket here.
const (
	BucketPass            Bucket = "pass"
	BucketFail            Bucket = "fail"
	BucketNotExpressible  Bucket = "not-expressible"
	BucketDiffersByDesign Bucket = "differs-by-design"
)

// Buckets lists every bucket in report order.
var Buckets = []Bucket{BucketPass, BucketFail, BucketNotExpressible, BucketDiffersByDesign}

// Expressibility is what the classifier decides about an activity before any
// translation exists, from the constructs it uses and the implementation's
// trace. A construct with no SysML v2 spelling makes it not expressible
// whatever else it uses; an action the implementation fired once per object
// token is the documented design difference; otherwise a run decides.
type Expressibility int

// The three classes, in decision order.
const (
	// NotExpressible: the activity uses a construct with no SysML v2 spelling.
	NotExpressible Expressibility = iota
	// DiffersByDesign: spellable, but its outputs depend on fUML's per-token
	// re-firing, which SysML v2 performs once with every delivery.
	DiffersByDesign
	// Expressible: spellable, and a run decides between pass and fail.
	Expressible
)

var expressibilityNames = [...]string{
	NotExpressible:  "not-expressible",
	DiffersByDesign: "differs-by-design",
	Expressible:     "expressible",
}

func (c Expressibility) String() string {
	if c >= 0 && int(c) < len(expressibilityNames) {
		return expressibilityNames[c]
	}
	return fmt.Sprintf("Expressibility(%d)", int(c))
}

// Bucket is the bucket the class fixes before a run, or false when only a run
// can decide between pass and fail.
func (c Expressibility) Bucket() (Bucket, bool) {
	switch c {
	case NotExpressible:
		return BucketNotExpressible, true
	case DiffersByDesign:
		return BucketDiffersByDesign, true
	}
	return "", false
}

// Construct is one fUML construct that decides a class, named as the construct
// map in docs/project/fuml-referee.md names it.
type Construct string

// The deciding constructs. Constructs with a spelling (control and object flow,
// fork, join, merge, decision, value specification, calls of activities and of
// library functions with a counterpart, object creation, feature reads and
// writes on a class, signals, accept events, structured nodes, read self) are
// never recorded.
const (
	ConstructExceptionModel   Construct = "exception-test model"
	ConstructCallOperation    Construct = "CallOperationAction"
	ConstructAcceptCall       Construct = "AcceptCallAction"
	ConstructReply            Construct = "ReplyAction"
	ConstructReadExtent       Construct = "ReadExtentAction"
	ConstructReadIsClassified Construct = "ReadIsClassifiedObjectAction"
	ConstructTestIdentity     Construct = "TestIdentityAction"
	ConstructReclassify       Construct = "ReclassifyObjectAction"
	ConstructUnmarshall       Construct = "UnmarshallAction"
	ConstructReadLink         Construct = "ReadLinkAction"
	ConstructAssociationEnd   Construct = "structural feature action on an association end"
	ConstructDestroyObject    Construct = "DestroyObjectAction"
	ConstructRaiseException   Construct = "RaiseExceptionAction"
	ConstructExceptionHandler Construct = "ExceptionHandler"
	ConstructCentralBuffer    Construct = "CentralBufferNode"
	ConstructDataStore        Construct = "DataStoreNode"
	ConstructUnknownNode      Construct = "node kind outside the construct map"
	ConstructUnlimitedNatural Construct = "UnlimitedNatural"
	ConstructLibraryGap       Construct = "library behavior without a KerML counterpart"
	ConstructUnresolvedCall   Construct = "call of an unresolved behavior"
	ConstructDependency       Construct = "dependency on a not-expressible behavior"
	ConstructPerTokenRefiring Construct = "per-token action re-firing"
)

// constructs gives each deciding construct its class and the one-line reason a
// report spells beside the places it was used.
var constructs = map[Construct]struct {
	class Expressibility
	why   string
}{
	ConstructExceptionModel:   {NotExpressible, "the exception-test model exists to raise and handle exceptions, which SysML v2 actions cannot spell"},
	ConstructCallOperation:    {NotExpressible, "SysML v2 actions have no operation-call semantics"},
	ConstructAcceptCall:       {NotExpressible, "SysML v2 actions have no operation-call semantics"},
	ConstructReply:            {NotExpressible, "SysML v2 actions have no operation-call semantics"},
	ConstructReadExtent:       {NotExpressible, "SysML v2 has no classifier extent"},
	ConstructReadIsClassified: {NotExpressible, "SysML v2 actions cannot test an object's classifiers"},
	ConstructTestIdentity:     {NotExpressible, "SysML v2 actions cannot compare object identity"},
	ConstructReclassify:       {NotExpressible, "SysML v2 objects cannot change classifier"},
	ConstructUnmarshall:       {NotExpressible, "SysML v2 actions have no unmarshalling"},
	ConstructReadLink:         {NotExpressible, "SysML v2 actions have no association links"},
	ConstructAssociationEnd:   {NotExpressible, "SysML v2 actions have no association links"},
	ConstructDestroyObject:    {NotExpressible, "SysML v2 actions cannot destroy an object"},
	ConstructRaiseException:   {NotExpressible, "SysML v2 actions cannot raise exceptions"},
	ConstructExceptionHandler: {NotExpressible, "SysML v2 actions cannot handle exceptions"},
	ConstructCentralBuffer:    {NotExpressible, "SysML v2 actions have no buffer node"},
	ConstructDataStore:        {NotExpressible, "SysML v2 actions have no buffer node"},
	ConstructUnknownNode:      {NotExpressible, "the construct map gives it no spelling"},
	ConstructUnlimitedNatural: {NotExpressible, "KerML's ScalarValues has no unlimited natural"},
	ConstructLibraryGap:       {NotExpressible, "KerML's function library has no counterpart"},
	ConstructUnresolvedCall:   {NotExpressible, "the called behavior is not declared in the model or the library"},
	ConstructDependency:       {NotExpressible, "a behavior it calls or starts is itself not expressible"},
	ConstructPerTokenRefiring: {DiffersByDesign, "fUML fires an action once per object token on a multiplicity-1 pin; SysML v2 performs the node once with every delivery"},
}

// nodeConstructs maps the node kinds with no SysML v2 spelling to their construct.
var nodeConstructs = map[NodeKind]Construct{
	CallOperationAction:          ConstructCallOperation,
	AcceptCallAction:             ConstructAcceptCall,
	ReplyAction:                  ConstructReply,
	ReadExtentAction:             ConstructReadExtent,
	ReadIsClassifiedObjectAction: ConstructReadIsClassified,
	TestIdentityAction:           ConstructTestIdentity,
	ReclassifyObjectAction:       ConstructReclassify,
	UnmarshallAction:             ConstructUnmarshall,
	ReadLinkAction:               ConstructReadLink,
	DestroyObjectAction:          ConstructDestroyObject,
	RaiseExceptionAction:         ConstructRaiseException,
	CentralBufferNode:            ConstructCentralBuffer,
	DataStoreNode:                ConstructDataStore,
}

// spellableKinds are the node kinds the construct map gives a SysML v2 spelling.
// A FlowFinalNode is spelled by omission: a token with no successor is consumed.
var spellableKinds = map[NodeKind]bool{
	InputPin: true, OutputPin: true, ActivityParameterNode: true,
	InitialNode: true, ActivityFinalNode: true, FlowFinalNode: true, ForkNode: true,
	JoinNode: true, MergeNode: true, DecisionNode: true, StructuredActivityNode: true,
	ValueSpecificationAction: true, CallBehaviorAction: true, CreateObjectAction: true,
	ReadSelfAction: true, ReadStructuralFeatureAction: true,
	AddStructuralFeatureValueAction: true, RemoveStructuralFeatureValueAction: true,
	SendSignalAction: true, AcceptEventAction: true, StartObjectBehaviorAction: true,
}

// unlimitedNaturalType is the fUML primitive type KerML's ScalarValues lack.
const unlimitedNaturalType = "UnlimitedNatural"

// positionRoles are the pins that take a position, not a value: `*` there
// means "at the end", which needs no unlimited natural to spell.
var positionRoles = map[string]bool{"insertAt": true, "removeAt": true}

// Use is one occurrence of a deciding construct: which, and where.
type Use struct {
	Construct Construct
	// Where names the node, pin, parameter or behavior using it.
	Where string
}

// Refire is one action the implementation fired more than once within one
// execution of its activity.
type Refire struct {
	// Activity is the activity the action belongs to: the classified activity
	// or one it calls. The ids are the XMI ids of the activity and the node.
	Activity   string
	ActivityID string
	Action     string
	ActionID   string
	Fires      int
}

func (r Refire) String() string {
	return fmt.Sprintf("%s.%s fired %d times", r.Activity, r.Action, r.Fires)
}

// Classification is an activity's class and the construct uses that decided it.
type Classification struct {
	Class Expressibility
	// Uses lists every recorded construct use, those deciding the class first,
	// then in document order. An Expressible activity has none.
	Uses []Use
}

// Reason spells the uses that decided the class, one clause per construct.
func (c Classification) Reason() string {
	if c.Class == Expressible {
		return "every construct has a SysML v2 spelling"
	}
	var order []Construct
	where := map[Construct][]string{}
	for _, u := range c.Uses {
		if constructs[u.Construct].class != c.Class {
			continue
		}
		if _, seen := where[u.Construct]; !seen {
			order = append(order, u.Construct)
		}
		where[u.Construct] = append(where[u.Construct], u.Where)
	}
	clauses := make([]string, 0, len(order))
	for _, k := range order {
		clauses = append(clauses, fmt.Sprintf("%s (%s): %s", k, constructs[k].why, strings.Join(where[k], ", ")))
	}
	return strings.Join(clauses, "; ")
}

// decisive names the constructs that decided the class, each once, a
// dependency by the behavior's name and its own decisive constructs.
func (c Classification) decisive() string {
	var names []string
	seen := map[string]bool{}
	for _, u := range c.Uses {
		if constructs[u.Construct].class != c.Class {
			continue
		}
		name := string(u.Construct)
		if u.Construct == ConstructDependency {
			name = u.Where
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// Classify decides an activity's class from the constructs it and the
// behaviors it calls or starts use and, when the implementation's record of
// it is given, from the actions that record shows firing once per object token.
func Classify(a *Activity, x *ExpectedActivity) Classification {
	return classify(a, x, map[*Activity]bool{})
}

// classify is Classify over the activities not in visiting, which guards
// against mutually recursive behaviors.
func classify(a *Activity, x *ExpectedActivity, visiting map[*Activity]bool) Classification {
	visiting[a] = true
	defer delete(visiting, a)
	var uses []Use
	add := func(c Construct, where string) { uses = append(uses, Use{c, where}) }
	if a.Model != nil && a.Model.Exception() {
		add(ConstructExceptionModel, a.Name+" is declared in "+ExceptionTestsFile)
	}
	for _, p := range a.Parameters {
		if p.Type.Name == unlimitedNaturalType {
			add(ConstructUnlimitedNatural, "parameter "+p.Name)
		}
	}
	for _, n := range a.AllNodes() {
		classifyNode(n, add)
	}
	for _, d := range dependencies(a) {
		if visiting[d] {
			continue
		}
		if c := classify(d, nil, visiting); c.Class == NotExpressible {
			add(ConstructDependency, d.Name+" ("+c.decisive()+")")
		}
	}
	if x != nil {
		for _, r := range x.Refired() {
			if objectFed(a, r) {
				add(ConstructPerTokenRefiring, r.String())
			}
		}
	}
	class := Expressible
	for _, u := range uses {
		if c := constructs[u.Construct].class; c < class {
			class = c
		}
	}
	sort.SliceStable(uses, func(i, j int) bool {
		return constructs[uses[i].Construct].class < constructs[uses[j].Construct].class
	})
	return Classification{Class: class, Uses: uses}
}

// classifyNode records the deciding constructs one node uses.
func classifyNode(n *Node, add func(Construct, string)) {
	if c, ok := nodeConstructs[n.Kind]; ok {
		add(c, n.Label())
	} else if !spellableKinds[n.Kind] {
		add(ConstructUnknownNode, n.Label()+" is a "+string(n.Kind))
	}
	for range n.Handlers {
		add(ConstructExceptionHandler, n.Label())
	}
	if n.Kind.Pin() && n.Type.Name == unlimitedNaturalType && !positionRoles[n.Role] && !feedsPosition(n) {
		add(ConstructUnlimitedNatural, "pin "+pinLabel(n))
	}
	if n.Feature != nil && n.Feature.Association != nil {
		add(ConstructAssociationEnd, n.Label())
	}
	if n.Kind == CallBehaviorAction {
		classifyCall(n, add)
	}
}

// classifyCall records what a CallBehaviorAction needs that the notation lacks.
func classifyCall(n *Node, add func(Construct, string)) {
	b := n.Behavior
	switch {
	case b == nil:
		add(ConstructUnresolvedCall, n.Label()+" names no behavior")
	case b.External:
		if _, ok := LibraryCounterpart(b.Name); !ok {
			add(ConstructLibraryGap, n.Label()+" calls "+b.Name)
		}
	case b.Activity == nil:
		add(ConstructUnresolvedCall, n.Label()+" calls "+b.Name)
	}
}

// feedsPosition reports whether an output pin flows only into position pins.
func feedsPosition(n *Node) bool {
	if n.Kind != OutputPin || len(n.Outgoing) == 0 {
		return false
	}
	for _, e := range n.Outgoing {
		if e.Target == nil || !positionRoles[e.Target.Role] {
			return false
		}
	}
	return true
}

// pinLabel names a pin by its name, or by its owner and role when unnamed.
func pinLabel(n *Node) string {
	if n.Name != "" || n.Owner == nil {
		return n.Label()
	}
	return n.Owner.Label() + "." + n.Role
}

// dependencies lists the model's activities the activity runs: the activities
// it calls and the classifier behaviors of the objects it starts, each once,
// in document order. Creating an object runs nothing; the activity itself is
// never listed.
func dependencies(a *Activity) []*Activity {
	var deps []*Activity
	seen := map[*Activity]bool{a: true}
	take := func(d *Activity) {
		if d != nil && !seen[d] {
			seen[d] = true
			deps = append(deps, d)
		}
	}
	for _, n := range a.AllNodes() {
		switch n.Kind {
		case CallBehaviorAction:
			if n.Behavior != nil {
				take(n.Behavior.Activity)
			}
		case StartObjectBehaviorAction:
			for _, p := range n.Inputs() {
				if p.Role == "object" {
					types, _ := objectTypes(p, map[*Node]*typeSources{})
					for _, t := range types {
						take(classifierBehavior(a.Model, t))
					}
				}
			}
		}
	}
	return deps
}

// typeSources is what objectTypes found for one node; done is false while the
// node is still being searched, which only a flow cycle leads back to.
type typeSources struct {
	types    []TypeRef
	resolved bool
	done     bool
}

// objectTypes lists the types the objects reaching a pin may have, tracing its
// object flows back to their sources; the node's own type stands in for a path
// with none. Each node is searched once, so reconverging paths share its result.
func objectTypes(n *Node, found map[*Node]*typeSources) (types []TypeRef, resolved bool) {
	if s, ok := found[n]; ok {
		if !s.done {
			return nil, true
		}
		return s.types, s.resolved
	}
	s := &typeSources{}
	found[n] = s
	defer func() { s.types, s.resolved, s.done = types, resolved, true }()
	if n.Kind == OutputPin && n.Owner != nil && n.Owner.Kind == CreateObjectAction && !n.Owner.Classifier.Zero() {
		return []TypeRef{n.Owner.Classifier}, true
	}
	fed, resolved := false, true
	for _, e := range n.Incoming {
		if e.Kind == ObjectFlow && e.Source != nil {
			sources, ok := objectTypes(e.Source, found)
			fed, resolved = fed || len(sources) > 0, resolved && ok
			types = append(types, sources...)
		}
	}
	if fed && resolved {
		return types, true
	}
	if !n.Type.Zero() {
		return append(types, n.Type), true
	}
	return types, false
}

// classifierBehavior is the behavior an object of the type runs when started:
// an active activity runs itself, a class its classifierBehavior.
func classifierBehavior(m *Model, t TypeRef) *Activity {
	if m == nil {
		return nil
	}
	if act := m.Activity(t.ID); act != nil {
		return act
	}
	if c := m.Class(t.ID); c != nil {
		return c.ClassifierBehavior
	}
	return nil
}

// objectFed reports whether the refired action takes an object flow into a
// pin: the re-firing SysML v2 lacks, a node performing once with every
// delivery (action_node_concurrent_performances). An action fed by control
// tokens alone, as through a merge, re-fires in SysML v2 too.
func objectFed(a *Activity, r Refire) bool {
	target := a
	if r.ActivityID != a.ID {
		if a.Model == nil {
			return false
		}
		if target = a.Model.Activity(r.ActivityID); target == nil {
			return false
		}
	}
	n := target.Node(r.ActionID)
	if n == nil {
		return false
	}
	for _, p := range n.Inputs() {
		for _, e := range p.Incoming {
			if e.Kind == ObjectFlow {
				return true
			}
		}
	}
	return false
}

// libraryCounterparts maps each fUML library behavior the models call to the
// KerML function the emitter spells it as; a library behavior absent here has
// no counterpart and makes its activity not expressible.
var libraryCounterparts = map[string]string{
	"PrimitiveBehaviors::IntegerFunctions::+":   "IntegerFunctions::'+'",
	"PrimitiveBehaviors::IntegerFunctions::-":   "IntegerFunctions::'-'",
	"PrimitiveBehaviors::IntegerFunctions::*":   "IntegerFunctions::'*'",
	"PrimitiveBehaviors::IntegerFunctions::Neg": "IntegerFunctions::'-' (unary)",
	// Div truncates toward zero, as narrowing the rational quotient does.
	"PrimitiveBehaviors::IntegerFunctions::Div":      "RationalFunctions::ToInteger(IntegerFunctions::'/')",
	"PrimitiveBehaviors::IntegerFunctions::Mod":      "IntegerFunctions::'%'",
	"PrimitiveBehaviors::IntegerFunctions::Abs":      "IntegerFunctions::abs",
	"PrimitiveBehaviors::IntegerFunctions::Max":      "IntegerFunctions::max",
	"PrimitiveBehaviors::IntegerFunctions::Min":      "IntegerFunctions::min",
	"PrimitiveBehaviors::IntegerFunctions::<":        "IntegerFunctions::'<'",
	"PrimitiveBehaviors::IntegerFunctions::<=":       "IntegerFunctions::'<='",
	"PrimitiveBehaviors::IntegerFunctions::>=":       "IntegerFunctions::'>='",
	"PrimitiveBehaviors::RealFunctions::+":           "RealFunctions::'+'",
	"PrimitiveBehaviors::RealFunctions::-":           "RealFunctions::'-'",
	"PrimitiveBehaviors::RealFunctions::*":           "RealFunctions::'*'",
	"PrimitiveBehaviors::RealFunctions::/":           "RealFunctions::'/'",
	"PrimitiveBehaviors::RealFunctions::Neg":         "RealFunctions::'-' (unary)",
	"PrimitiveBehaviors::RealFunctions::Inv":         "RealFunctions::'/' (1.0 / x)",
	"PrimitiveBehaviors::RealFunctions::Abs":         "RealFunctions::abs",
	"PrimitiveBehaviors::RealFunctions::Floor":       "RealFunctions::floor",
	"PrimitiveBehaviors::RealFunctions::Round":       "RealFunctions::round",
	"PrimitiveBehaviors::RealFunctions::Max":         "RealFunctions::max",
	"PrimitiveBehaviors::RealFunctions::Min":         "RealFunctions::min",
	"PrimitiveBehaviors::RealFunctions::<":           "RealFunctions::'<'",
	"PrimitiveBehaviors::RealFunctions::<=":          "RealFunctions::'<='",
	"PrimitiveBehaviors::RealFunctions::>":           "RealFunctions::'>'",
	"PrimitiveBehaviors::RealFunctions::>=":          "RealFunctions::'>='",
	"PrimitiveBehaviors::RealFunctions::ToInteger":   "RealFunctions::ToInteger",
	"PrimitiveBehaviors::BooleanFunctions::And":      "BooleanFunctions::'&'",
	"PrimitiveBehaviors::BooleanFunctions::Or":       "BooleanFunctions::'|'",
	"PrimitiveBehaviors::BooleanFunctions::Not":      "BooleanFunctions::'not'",
	"PrimitiveBehaviors::BooleanFunctions::Xor":      "BooleanFunctions::'xor'",
	"PrimitiveBehaviors::BooleanFunctions::Implies":  "ControlFunctions::'implies'",
	"PrimitiveBehaviors::StringFunctions::Concat":    "StringFunctions::'+'",
	"PrimitiveBehaviors::StringFunctions::Size":      "StringFunctions::Length",
	"PrimitiveBehaviors::StringFunctions::Substring": "StringFunctions::Substring",
	"PrimitiveBehaviors::ListFunctions::ListSize":    "SequenceFunctions::size",
	"PrimitiveBehaviors::ListFunctions::ListGet":     "SequenceFunctions::'#'",
	"PrimitiveBehaviors::ListFunctions::ListConcat":  "SequenceFunctions::union",
}

// LibraryCounterpart names the KerML function the emitter spells a fUML
// library behavior as, by the behavior's qualified name.
func LibraryCounterpart(qualified string) (string, bool) {
	kerml, ok := libraryCounterparts[qualified]
	return kerml, ok
}
