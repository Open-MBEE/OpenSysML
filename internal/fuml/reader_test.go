package fuml

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// suite is the pinned suite as read: the library and both models.
type suite struct {
	lib       *Library
	tests     *Model
	exception *Model
}

// loadSuite reads the pinned suite, skipping when it is absent unless
// RequireEnv is set, in which case absence is a failure as CI demands.
func loadSuite(t *testing.T) *suite {
	t.Helper()
	root, err := Locate(repoRoot)
	if err != nil {
		if errors.Is(err, ErrSuiteAbsent) && !Required() {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	lib, err := ReadLibraryFile(filepath.Join(root, LibraryFile))
	if err != nil {
		t.Fatal(err)
	}
	s := &suite{lib: lib}
	for _, m := range []struct {
		file string
		into **Model
	}{{TestsFile, &s.tests}, {ExceptionTestsFile, &s.exception}} {
		if *m.into, err = ReadModelFile(filepath.Join(root, m.file), lib); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// activity fails the test when the model does not declare the activity once.
func activity(t *testing.T, m *Model, name string) *Activity {
	t.Helper()
	a := m.ActivityNamed(name)
	if a == nil {
		t.Fatalf("%s declares no single activity %q", m.File, name)
	}
	return a
}

// node fails the test when the activity does not name the node once.
func node(t *testing.T, a *Activity, name string) *Node {
	t.Helper()
	n := a.NodeNamed(name)
	if n == nil {
		t.Fatalf("%s has no single node %q", a.Name, name)
	}
	return n
}

func countNodes(m *Model) (kinds map[NodeKind]int, edges map[EdgeKind]int) {
	kinds, edges = map[NodeKind]int{}, map[EdgeKind]int{}
	for _, a := range m.Activities {
		for _, n := range a.AllNodes() {
			kinds[n.Kind]++
		}
		for _, e := range a.Edges {
			edges[e.Kind]++
		}
	}
	return kinds, edges
}

// TestSuiteRead pins the shape of the pinned models: how many activities,
// classifiers, nodes and edges each declares, read without a diagnostic and
// with every edge, pin, parameter node and call resolved.
func TestSuiteRead(t *testing.T) {
	s := loadSuite(t)
	for _, m := range []*Model{s.tests, s.exception} {
		for _, d := range m.Diagnostics {
			t.Errorf("%s: %s", m.File, d)
		}
		for _, a := range m.Activities {
			if m.Activity(a.ID) != a {
				t.Errorf("%s: %s is not found by its id %s", m.File, a.Name, a.ID)
			}
			for _, e := range a.Edges {
				if e.Source == nil || e.Target == nil {
					t.Errorf("%s: %s edge %s has an unresolved end", m.File, a.Name, e.ID)
				}
			}
			for _, n := range a.AllNodes() {
				if n.Activity != a {
					t.Errorf("%s: %s node %s belongs to another activity", m.File, a.Name, n.Label())
				}
				if n.Kind == ActivityParameterNode && n.Parameter == nil {
					t.Errorf("%s: %s parameter node %s names no parameter", m.File, a.Name, n.Label())
				}
				if n.Kind.Pin() && n.Owner == nil {
					t.Errorf("%s: %s pin %s has no owner", m.File, a.Name, n.Label())
				}
				if n.Kind == CallBehaviorAction && (n.Behavior == nil || (!n.Behavior.External && n.Behavior.Activity == nil)) {
					t.Errorf("%s: %s call %s resolves to no behavior", m.File, a.Name, n.Label())
				}
			}
		}
	}

	// 42 packaged activities plus ActiveClass's classifier behavior; the 44th
	// uml:Activity element in the file is an href to the library's WriteLine.
	if got := len(s.tests.Activities); got != 43 {
		t.Errorf("%s: activities = %d, want 43", TestsFile, got)
	}
	if got := len(s.exception.Activities); got != 12 {
		t.Errorf("%s: activities = %d, want 12", ExceptionTestsFile, got)
	}
	want := []struct {
		m                              *Model
		classes, signals, associations int
		control, object                int
	}{
		{s.tests, 7, 3, 2, 95, 345},
		{s.exception, 4, 0, 0, 22, 93},
	}
	for _, w := range want {
		kinds, edges := countNodes(w.m)
		if len(w.m.Classes) != w.classes || len(w.m.Signals) != w.signals || len(w.m.Associations) != w.associations {
			t.Errorf("%s: %d classes, %d signals, %d associations; want %d, %d, %d", w.m.File,
				len(w.m.Classes), len(w.m.Signals), len(w.m.Associations), w.classes, w.signals, w.associations)
		}
		if edges[ControlFlow] != w.control || edges[ObjectFlow] != w.object {
			t.Errorf("%s: %d control and %d object flows; want %d and %d", w.m.File,
				edges[ControlFlow], edges[ObjectFlow], w.control, w.object)
		}
		for k := range kinds {
			if !k.Pin() && !k.Control() && !k.Object() && k != StructuredActivityNode && nodeConstructs[k] == "" && !spellableKinds[k] {
				t.Errorf("%s: node kind %s is unknown to the classifier", w.m.File, k)
			}
		}
	}
	kinds, _ := countNodes(s.tests)
	for k, n := range map[NodeKind]int{
		InputPin: 186, OutputPin: 191, ActivityParameterNode: 113, CallBehaviorAction: 73,
		ValueSpecificationAction: 62, ForkNode: 39, AddStructuralFeatureValueAction: 22,
		CreateObjectAction: 10, StructuredActivityNode: 2, CentralBufferNode: 1, DataStoreNode: 1,
	} {
		if kinds[k] != n {
			t.Errorf("%s: %d %s nodes, want %d", TestsFile, kinds[k], k, n)
		}
	}
	kinds, _ = countNodes(s.exception)
	if kinds[RaiseExceptionAction] != 8 || kinds[StructuredActivityNode] != 18 {
		t.Errorf("%s: %d raise actions in %d structured nodes, want 8 in 18",
			ExceptionTestsFile, kinds[RaiseExceptionAction], kinds[StructuredActivityNode])
	}
}

// TestSuiteReadsControlAndObjectFlow checks the pilot activities' topology:
// an object-fed decision with guards, a join fed by two flows, a parameter node behind a
// multi-valued output, and library calls resolved to qualified names.
func TestSuiteReadsControlAndObjectFlow(t *testing.T) {
	s := loadSuite(t)

	dj := activity(t, s.tests, "DecisionJoin")
	decision := node(t, dj, "DecisionNode")
	if len(decision.Outgoing) != 2 {
		t.Fatalf("DecisionJoin's decision has %d outgoing edges, want 2", len(decision.Outgoing))
	}
	var guards []string
	for _, e := range decision.Outgoing {
		if e.Kind != ObjectFlow || e.Target != node(t, dj, "JoinNode") {
			t.Errorf("DecisionJoin's decision edge %s is a %s to %s", e.ID, e.Kind, e.Target.Label())
		}
		guards = append(guards, e.Guard.String())
	}
	if strings.Join(guards, ", ") != "LiteralInteger (default), LiteralInteger 1" {
		t.Errorf("DecisionJoin's guards = %v", guards)
	}
	join := node(t, dj, "JoinNode")
	if len(join.Incoming) != 2 || len(join.Outgoing) != 1 {
		t.Errorf("DecisionJoin's join has %d in, %d out", len(join.Incoming), len(join.Outgoing))
	}

	fm := activity(t, s.tests, "ForkMerge")
	if len(fm.Parameters) != 1 || fm.Parameters[0].Direction != Out || fm.Parameters[0].String() != "[0..*]" {
		t.Errorf("ForkMerge's parameters = %+v", fm.Parameters)
	}
	value := node(t, fm, "Value(0)")
	if value.Value.String() != "LiteralInteger (default)" || len(value.Outputs()) != 1 {
		t.Errorf("ForkMerge's Value(0) = %s with %d outputs", value.Value, len(value.Outputs()))
	}
	if out := value.Outputs()[0]; len(out.Outgoing) != 1 || out.Outgoing[0].Kind != ObjectFlow ||
		out.Outgoing[0].Target.Kind != ActivityParameterNode || out.Outgoing[0].Target.Parameter != fm.Parameters[0] {
		t.Errorf("ForkMerge's result pin does not flow to the output parameter node")
	}
	if len(value.Incoming) != 1 || value.Incoming[0].Kind != ControlFlow || value.Incoming[0].Source != node(t, fm, "MergeNode") {
		t.Errorf("ForkMerge's Value(0) is not enabled by the merge")
	}

	ti := activity(t, s.tests, "TestIntegerFunctions")
	plus := node(t, ti, "Call(Plus)")
	if plus.Behavior == nil || !plus.Behavior.External || plus.Behavior.Name != "PrimitiveBehaviors::IntegerFunctions::+" || plus.Behavior.Kind != "FunctionBehavior" {
		t.Errorf("Call(Plus) calls %+v", plus.Behavior)
	}
	if ins := plus.Inputs(); len(ins) != 2 || ins[0].Type.Name != "Integer" || !ins[0].Type.External || ins[0].Multiplicity.String() != "[1..1]" {
		t.Errorf("Call(Plus) inputs = %+v", ins)
	}
	div := node(t, ti, "Call(Div)")
	if outs := div.Outputs(); len(outs) != 1 || outs[0].Multiplicity.String() != "[0..1]" {
		t.Errorf("Call(Div) outputs = %+v", outs)
	}

	cc := activity(t, s.tests, "CopierCaller")
	call := node(t, cc, "Call(Copier)")
	if call.Behavior == nil || call.Behavior.External || call.Behavior.Activity != activity(t, s.tests, "Copier") {
		t.Errorf("CopierCaller's call resolves to %+v", call.Behavior)
	}

	tb := activity(t, s.tests, "TestBooleanFunctions")
	if p := tb.Parameters[0]; p.Name != "NotResult" || !p.Ordered || p.String() != "[0..*] ordered" {
		t.Errorf("TestBooleanFunctions' first parameter = %+v", p)
	}

	ne := activity(t, s.tests, "NodeEnabler")
	structured := node(t, ne, "StructuredNode")
	if structured.Kind != StructuredActivityNode || len(structured.Nodes) != 1 || structured.Nodes[0].Owner != structured {
		t.Errorf("NodeEnabler's structured node = %+v", structured)
	}

	tds := activity(t, s.tests, "TestDataStore")
	if d := node(t, tds, "DecisionNode"); d.DecisionInputFlow == nil || d.DecisionInputFlow.Kind != ObjectFlow {
		t.Errorf("TestDataStore's decision has no object decision input flow")
	}
}

// TestSuiteReadsClassifiers checks the object model: classes with
// generalizations, attributes and a classifier behavior, signals, association
// ends known to their properties, and operations owned by classes and by the
// activity whose accept-call action serves them.
func TestSuiteReadsClassifiers(t *testing.T) {
	s := loadSuite(t)
	var classes []string
	for _, c := range s.tests.Classes {
		classes = append(classes, c.Name)
		if s.tests.Class(c.ID) != c {
			t.Errorf("class %s is not found by its id", c.Name)
		}
	}
	if got := strings.Join(classes, " "); got != "Specific General TestClass Subclass1 Subclass2 ActiveClass TestComposite" {
		t.Errorf("classes = %s", got)
	}

	var sub1 *Class
	for _, c := range s.tests.Classes {
		if c.Name == "Subclass1" {
			sub1 = c
		}
	}
	if len(sub1.Generals) != 1 || sub1.Generals[0].Name != "TestClass" || sub1.Generals[0].Kind != "Class" {
		t.Errorf("Subclass1 generalizes %v", sub1.Generals)
	}
	tc := s.tests.Class(sub1.Generals[0].ID)
	if tc == nil || len(tc.Attributes) != 2 {
		t.Fatalf("TestClass = %+v", tc)
	}
	if y := tc.Attributes[1]; y.Name != "y" || y.Type.Name != "Integer" || y.String() != "[0..*] ordered nonunique" || y.Owner.Name != "TestClass" {
		t.Errorf("TestClass.y = %+v", y)
	}

	active := activity(t, s.tests, "ActiveClassBehavior")
	if active.Owner == nil || active.Owner.Name != "ActiveClass" || !active.Owner.Active || active.Owner.ClassifierBehavior != active {
		t.Errorf("ActiveClassBehavior is owned by %+v", active.Owner)
	}
	accept := node(t, active, "Accept(TestSignal)")
	if len(accept.Triggers) != 1 || accept.Triggers[0].Signal.Name != "TestSignal" || accept.Triggers[0].Signal.Kind != "Signal" {
		t.Errorf("Accept(TestSignal) triggers = %+v", accept.Triggers)
	}

	var signals []string
	for _, sig := range s.tests.Signals {
		signals = append(signals, sig.Name)
		if sig.Name == "SpecializedSignal" && (len(sig.Generals) != 1 || sig.Generals[0].Name != "TestSignal") {
			t.Errorf("SpecializedSignal generalizes %v", sig.Generals)
		}
	}
	if got := strings.Join(signals, " "); got != "TestSignal SpecializedSignal OtherSignal" {
		t.Errorf("signals = %s", got)
	}

	for _, assoc := range s.tests.Associations {
		if len(assoc.Ends) != 2 {
			t.Errorf("%s has %d ends", assoc.Name, len(assoc.Ends))
		}
		for _, end := range assoc.Ends {
			if end.Association != assoc {
				t.Errorf("%s.%s does not know its association", assoc.Name, end.Name)
			}
		}
	}
	writer := activity(t, s.tests, "TestAssociationEndWriterReader")
	if add := node(t, writer, "Add(end2)-1"); add.Feature == nil || add.Feature.Association == nil || add.Feature.Association.Name != "TestAssociation" {
		t.Errorf("Add(end2)-1 writes %+v", add.Feature)
	}
	destroyer := activity(t, s.tests, "TestCompositeObjectDestroyer")
	if read := node(t, destroyer, "Read(composite)"); len(read.Ends) != 2 || read.Ends[0].End == nil || read.Ends[1].End == nil {
		t.Errorf("Read(composite) ends = %+v", read.Ends)
	}

	accepter := activity(t, s.tests, "TestCallAccepter")
	call := node(t, accepter, "Accept(test)")
	if len(call.Triggers) != 1 || call.Triggers[0].Operation == nil || call.Triggers[0].Operation.Name != "test" {
		t.Fatalf("Accept(test) triggers = %+v", call.Triggers)
	}
	sender := activity(t, s.tests, "TestCallSender")
	if op := node(t, sender, "Call(test)"); op.Operation != call.Triggers[0].Operation {
		t.Errorf("Call(test) calls %+v, not the accepted operation", op.Operation)
	}

	var c *Class
	for _, cls := range s.exception.Classes {
		if cls.Name == "C" {
			c = cls
		}
	}
	if c == nil || len(c.Operations) != 2 || len(c.Behaviors) != 3 {
		t.Fatalf("exception class C = %+v", c)
	}
	if op := c.Operations[0]; op.Name != "raiseException" || op.Owner != c || len(op.Methods) != 1 || op.Methods[0].Name != "raiseException$Impl" || op.Methods[0].Owner != c {
		t.Errorf("C.raiseException = %+v", op)
	}
}

// TestSuiteReadsExceptionHandlers checks that the exception model's handlers
// and raise actions are read, so the classifier can name them.
func TestSuiteReadsExceptionHandlers(t *testing.T) {
	s := loadSuite(t)
	test004 := activity(t, s.exception, "Test004")
	raise := node(t, test004, "RaiseException")
	if raise.Kind != RaiseExceptionAction || len(raise.Handlers) != 2 {
		t.Fatalf("Test004's RaiseException = %s with %d handlers", raise.Kind, len(raise.Handlers))
	}
	for i, h := range raise.Handlers {
		if h.Protected != raise || h.Body == nil || h.Body.Kind != StructuredActivityNode || len(h.Types) != 1 || h.Types[0].Name != "Exception" {
			t.Errorf("handler %d = %+v", i, h)
		}
	}
	if !s.exception.Exception() || s.tests.Exception() {
		t.Errorf("Exception() = %v, %v", s.exception.Exception(), s.tests.Exception())
	}
}

const wrappedModel = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.1" xmlns:xmi="http://www.omg.org/spec/XMI/20110701" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML">
  <uml:Model xmi:id="m" name="Wrapped">
    <packagedElement xmi:type="uml:Activity" xmi:id="a" name="Doubler">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="p_in" name="x" direction="in">
        <type xmi:type="uml:PrimitiveType" href="pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#Integer"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="p_out" name="y" direction="out" isOrdered="true" isUnique="false">
        <type xmi:type="uml:PrimitiveType" href="pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="lv"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="uv" value="*"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="n_in" name="In" parameter="p_in"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="n_out" name="Out" parameter="p_out"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="call" name="Call(+)">
        <argument xmi:type="uml:InputPin" xmi:id="arg1" name="x"/>
        <argument xmi:type="uml:InputPin" xmi:id="arg2" name="y"/>
        <result xmi:type="uml:OutputPin" xmi:id="res" name="result"/>
        <behavior xmi:type="uml:FunctionBehavior" href="fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-plus"/>
      </node>
      <node xmi:type="uml:ForkNode" xmi:id="fork" name="Fork"/>
      <node xmi:type="uml:DecisionNode" xmi:id="dec" name="Decide" decisionInputFlow="f_dec"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="f1" source="n_in" target="fork"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="f2" source="fork" target="arg1"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="f3" source="fork" target="arg2"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="f4" source="res" target="dec"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="f_dec" source="fork" target="dec"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="f5" source="dec" target="n_out">
        <guard xmi:type="uml:LiteralBoolean" xmi:id="g" value="true"/>
      </edge>
      <edge xmi:type="uml:ControlFlow" xmi:id="c1" source="fork" target="missing"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="k" name="K">
      <ownedAttribute xmi:type="uml:Property" xmi:id="k_v" name="v" association="nowhere"/>
    </packagedElement>
  </uml:Model>
</xmi:XMI>
`

// TestReadModelInterpretsAWrappedDocument reads a hand-written model under an
// xmi:XMI root in an older XMI namespace: parameters with explicit bounds,
// pins under their action, edges attached to both ends, a decision input flow,
// a guard, an unresolved library href kept by fragment, and a diagnostic for
// each dangling reference.
func TestReadModelInterpretsAWrappedDocument(t *testing.T) {
	m, err := ReadModel(strings.NewReader(wrappedModel), nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Wrapped" || len(m.Activities) != 1 || len(m.Classes) != 1 {
		t.Fatalf("model = %+v", m)
	}
	a := m.Activities[0]
	if a.Parameters[0].String() != "[1..1]" || a.Parameters[0].Type.Name != "Integer" || !a.Parameters[0].Type.External {
		t.Errorf("x = %+v", a.Parameters[0])
	}
	if a.Parameters[1].String() != "[0..*] ordered nonunique" {
		t.Errorf("y = %s", a.Parameters[1])
	}
	call := node(t, a, "Call(+)")
	if len(call.Inputs()) != 2 || len(call.Outputs()) != 1 || call.Inputs()[0].Owner != call || call.Inputs()[0].Role != "argument" {
		t.Errorf("call pins = %+v", call.Pins)
	}
	if call.Behavior == nil || !call.Behavior.External || call.Behavior.Name != "PrimitiveBehaviors-IntegerFunctions-plus" || call.Behavior.Kind != "FunctionBehavior" {
		t.Errorf("call behavior = %+v", call.Behavior)
	}
	fork := node(t, a, "Fork")
	if len(fork.Incoming) != 1 || len(fork.Outgoing) != 4 || fork.Incoming[0].Source != node(t, a, "In") {
		t.Errorf("fork has %d in, %d out", len(fork.Incoming), len(fork.Outgoing))
	}
	dec := node(t, a, "Decide")
	if dec.DecisionInputFlow == nil || dec.DecisionInputFlow.ID != "f_dec" {
		t.Errorf("decision input flow = %+v", dec.DecisionInputFlow)
	}
	if out := node(t, a, "Out"); len(out.Incoming) != 1 || out.Incoming[0].Guard.String() != "LiteralBoolean true" || out.Parameter != a.Parameters[1] {
		t.Errorf("Out = %+v", out)
	}
	if len(a.Edges) != 7 {
		t.Errorf("edges = %d, want 7", len(a.Edges))
	}
	diags := strings.Join(m.Diagnostics, "\n")
	for _, want := range []string{"target missing is not a node", "association nowhere is not declared"} {
		if !strings.Contains(diags, want) {
			t.Errorf("diagnostics lack %q:\n%s", want, diags)
		}
	}
	if len(m.Diagnostics) != 2 {
		t.Errorf("diagnostics = %v", m.Diagnostics)
	}
}

func TestReadModelRejectsWhatIsNotAModel(t *testing.T) {
	if _, err := ReadModel(strings.NewReader("<a><b></a>"), nil); err == nil {
		t.Error("malformed XML read without error")
	}
	_, err := ReadModel(strings.NewReader(`<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><uml:Package xmlns:uml="u" xmi:id="p"/></xmi:XMI>`), nil)
	if err == nil || !strings.Contains(err.Error(), "not a uml:Model") {
		t.Errorf("err = %v, want a not-a-model error", err)
	}
}

// TestLibraryResolvesQualifiedNames reads the pinned library and names the
// behaviors the models call by their qualified names and kinds.
func TestLibraryResolvesQualifiedNames(t *testing.T) {
	s := loadSuite(t)
	for id, want := range map[string]string{
		"PrimitiveBehaviors-IntegerFunctions-plus":         "PrimitiveBehaviors::IntegerFunctions::+ FunctionBehavior",
		"PrimitiveBehaviors-ListFunctions-ListGet":         "PrimitiveBehaviors::ListFunctions::ListGet FunctionBehavior",
		"BasicInputOutput-WriteLine":                       "BasicInputOutput::WriteLine Activity",
		"PrimitiveBehaviors-UnlimitedNaturalFunctions-Max": "PrimitiveBehaviors::UnlimitedNaturalFunctions::Max FunctionBehavior",
	} {
		name, kind, ok := s.lib.Resolve(id)
		if !ok || name+" "+kind != want {
			t.Errorf("Resolve(%s) = %q %q %v, want %q", id, name, kind, ok, want)
		}
	}
	if _, _, ok := s.lib.Resolve("no-such-element"); ok {
		t.Error("an unknown id resolved")
	}
	var nilLib *Library
	if _, _, ok := nilLib.Resolve("PrimitiveBehaviors-IntegerFunctions-plus"); ok {
		t.Error("a nil library resolved")
	}
}
