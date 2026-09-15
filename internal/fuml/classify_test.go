package fuml

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// suiteClasses is the checklist: every activity of both pinned models with the
// class the classifier must give it. An expressible activity's bucket is
// decided by a run; the others are filed before any translation exists.
var suiteClasses = map[string]Expressibility{
	// fUML-Tests.uml: control and object flow, function libraries, objects.
	"Copier":                         Expressible,
	"CopierCaller":                   Expressible,
	"SimpleDecision":                 Expressible,
	"ForkJoin":                       Expressible,
	"ForkMerge":                      Expressible,
	"NodeEnabler":                    Expressible,
	"TestNodeEnabler":                Expressible,
	"TestIntegerFunctions":           Expressible,
	"TestIntegerComparisonFunctions": Expressible,
	"TestRealFunctions":              Expressible,
	"TestRealComparisonFunctions":    Expressible,
	"TestStringFunctions":            Expressible,
	"GenerateBooleanTestData":        Expressible,
	"GenerateListTestData":           Expressible,
	"TestListFunctions":              Expressible,
	"TestGeneralizationAssembly":     Expressible,
	"TestClassObjectCreator":         Expressible,
	"TestClassWriterReader":          Expressible,
	"TestClassAttributeWriter":       Expressible,
	"TestClassAttributeValueRemover": Expressible,
	"ActiveClassBehavior":            Expressible,
	"ActiveClassBehaviorSender":      Expressible,
	"TestSignalReceiver":             Expressible,
	"TestSpecializedSignalSend":      Expressible,

	// The implementation fires an action once per object token on a
	// multiplicity-1 pin; SysML v2 performs the node once with every delivery.
	"DecisionJoin":         DiffersByDesign,
	"ForkMergeData":        DiffersByDesign,
	"TestSimpleActivities": DiffersByDesign,
	"TestBooleanFunctions": DiffersByDesign,

	// No SysML v2 spelling.
	"HelloWorld":                     NotExpressible, // BasicInputOutput::WriteLine
	"TestUnlimitedNaturalFunctions":  NotExpressible, // UnlimitedNatural
	"TestClassIdentityTester":        NotExpressible, // TestIdentityAction
	"TestClassExtentReader":          NotExpressible, // ReadExtentAction
	"TestClassObjectDestroyer":       NotExpressible, // DestroyObjectAction
	"TestCompositeObjectDestroyer":   NotExpressible, // DestroyObjectAction, ReadLinkAction
	"TestClassReclassifier":          NotExpressible, // ReclassifyObjectAction
	"TestClassUnmarshaller":          NotExpressible, // UnmarshallAction
	"SelfReader":                     NotExpressible, // ReadIsClassifiedObjectAction
	"TestAssociationEndWriterReader": NotExpressible, // association ends
	"TestCentralBuffer":              NotExpressible, // CentralBufferNode
	"TestDataStore":                  NotExpressible, // DataStoreNode
	"TestCallAccepter":               NotExpressible, // AcceptCallAction, ReplyAction
	"TestCallSender":                 NotExpressible, // CallOperationAction
	"TestCallSend":                   NotExpressible, // starts TestCallSender

	// fUML-Exception-Tests.uml: every activity, translated never.
	"Test001": NotExpressible, "Test002": NotExpressible, "Test003": NotExpressible,
	"Test004": NotExpressible, "Test005": NotExpressible, "Test006": NotExpressible,
	"Test007": NotExpressible, "Test008": NotExpressible, "C_Factory": NotExpressible,
	"C$Impl": NotExpressible, "raiseException$Impl": NotExpressible, "CalledBehavior": NotExpressible,
}

// TestSuiteClassification classifies every activity of both pinned models
// against the checklist and pins the class counts: 24 expressible, 4 differing
// by design, 15 not expressible in the test model, all 12 of the exception model.
func TestSuiteClassification(t *testing.T) {
	s := loadSuite(t)
	_, e := committedExpected(t)
	type counts map[Expressibility]int
	got := map[string]counts{}
	seen := map[string]bool{}
	for _, m := range []*Model{s.tests, s.exception} {
		got[m.File] = counts{}
		for _, a := range m.Activities {
			c := Classify(a, e.Activity(m.File, a.ID))
			got[m.File][c.Class]++
			seen[a.Name] = true
			if want, ok := suiteClasses[a.Name]; !ok {
				t.Errorf("%s: %s is not on the checklist (%s: %s)", m.File, a.Name, c.Class, c.Reason())
			} else if c.Class != want {
				t.Errorf("%s: %s = %s, want %s: %s", m.File, a.Name, c.Class, want, c.Reason())
			}
			if c.Class == Expressible {
				if len(c.Uses) != 0 || c.Reason() != "every construct has a SysML v2 spelling" {
					t.Errorf("%s: expressible with uses %v", a.Name, c.Uses)
				}
				continue
			}
			if c.Reason() == "" || !strings.Contains(c.Reason(), "("+constructs[c.Uses[0].Construct].why+")") {
				t.Errorf("%s: %s without an actionable reason: %q", a.Name, c.Class, c.Reason())
			}
			if m.Exception() && (c.Class != NotExpressible || c.Uses[0].Construct != ConstructExceptionModel) {
				t.Errorf("%s: exception activity classified %s by %v", a.Name, c.Class, c.Uses)
			}
		}
	}
	for name := range suiteClasses {
		if !seen[name] {
			t.Errorf("checklist names %s, which neither model declares", name)
		}
	}
	if want := (counts{Expressible: 24, DiffersByDesign: 4, NotExpressible: 15}); !equalCounts(got[TestsFile], want) {
		t.Errorf("%s: %v, want %v", TestsFile, got[TestsFile], want)
	}
	if want := (counts{NotExpressible: 12}); !equalCounts(got[ExceptionTestsFile], want) {
		t.Errorf("%s: %v, want %v", ExceptionTestsFile, got[ExceptionTestsFile], want)
	}
}

func equalCounts(a, b map[Expressibility]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestSuiteClassificationReasons pins the reasons of the activities whose
// class is decided by something other than a node kind: the re-firing seen in
// the implementation's trace, a library gap, a dependency and a primitive type.
func TestSuiteClassificationReasons(t *testing.T) {
	s := loadSuite(t)
	_, e := committedExpected(t)
	reason := func(name string) string {
		a := activity(t, s.tests, name)
		return Classify(a, e.Activity(TestsFile, a.ID)).Reason()
	}
	for name, want := range map[string]string{
		"DecisionJoin":         "per-token action re-firing (fUML fires an action once per object token on a multiplicity-1 pin; SysML v2 performs the node once with every delivery): DecisionJoin.Action_A fired 2 times",
		"ForkMergeData":        "per-token action re-firing (fUML fires an action once per object token on a multiplicity-1 pin; SysML v2 performs the node once with every delivery): ForkMergeData.Action_B fired 2 times",
		"TestSimpleActivities": "per-token action re-firing (fUML fires an action once per object token on a multiplicity-1 pin; SysML v2 performs the node once with every delivery): DecisionJoin.Action_A fired 2 times, ForkMergeData.Action_B fired 2 times",
		"TestBooleanFunctions": "per-token action re-firing (fUML fires an action once per object token on a multiplicity-1 pin; SysML v2 performs the node once with every delivery): TestBooleanFunctions.Call(Not) fired 2 times, TestBooleanFunctions.Call(And) fired 4 times, TestBooleanFunctions.Call(Or) fired 4 times, TestBooleanFunctions.Call(Implies) fired 4 times, TestBooleanFunctions.Call(Xor) fired 4 times",
		"HelloWorld":           "library behavior without a KerML counterpart (KerML's function library has no counterpart): WriteLine calls BasicInputOutput::WriteLine",
		"TestCallSend":         "dependency on a not-expressible behavior (a behavior it calls or starts is itself not expressible): TestCallSender (CallOperationAction, TestCallAccepter (AcceptCallAction, ReplyAction))",
		"TestCallAccepter":     "AcceptCallAction (SysML v2 actions have no operation-call semantics): Accept(test); ReplyAction (SysML v2 actions have no operation-call semantics): Reply(test)",
		"TestCentralBuffer":    "CentralBufferNode (SysML v2 actions have no buffer node): CentralBufferNode",
	} {
		if got := reason(name); got != want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, want)
		}
	}
	// ForkMerge re-fires Value(0) through a merge, on control tokens alone, which
	// SysML v2 does too; the trace alone must not file it as differing.
	fm := activity(t, s.tests, "ForkMerge")
	x := e.Activity(TestsFile, fm.ID)
	if len(x.Refired()) != 1 || x.Refired()[0].Action != "Value(0)" {
		t.Fatalf("ForkMerge refired = %v", x.Refired())
	}
	if c := Classify(fm, x); c.Class != Expressible {
		t.Errorf("ForkMerge = %s: %s", c.Class, c.Reason())
	}
	// UnlimitedNatural on a position pin spells "at the end", so the list tests
	// stay expressible while the functions over the type do not.
	if got := reason("TestUnlimitedNaturalFunctions"); !strings.HasPrefix(got, "UnlimitedNatural (KerML's ScalarValues has no unlimited natural): parameter MaxResult, parameter MinResult, pin Value(3).result") {
		t.Errorf("TestUnlimitedNaturalFunctions: %s", got)
	}
	if got := reason("TestListFunctions"); got != "every construct has a SysML v2 spelling" {
		t.Errorf("TestListFunctions: %s", got)
	}
}

// TestSuiteLibraryCallsAreClassified checks that every library behavior the
// test model calls is either given a counterpart or named in a reason.
func TestSuiteLibraryCallsAreClassified(t *testing.T) {
	s := loadSuite(t)
	called := map[string]bool{}
	for _, a := range s.tests.Activities {
		for _, n := range a.AllNodes() {
			if n.Kind == CallBehaviorAction && n.Behavior != nil && n.Behavior.External {
				called[n.Behavior.Name] = true
			}
		}
	}
	var gaps []string
	for name := range called {
		if _, ok := LibraryCounterpart(name); !ok {
			gaps = append(gaps, name)
		}
	}
	sort.Strings(gaps)
	want := []string{
		"BasicInputOutput::WriteLine",
		"PrimitiveBehaviors::UnlimitedNaturalFunctions::<",
		"PrimitiveBehaviors::UnlimitedNaturalFunctions::<=",
		"PrimitiveBehaviors::UnlimitedNaturalFunctions::>",
		"PrimitiveBehaviors::UnlimitedNaturalFunctions::>=",
		"PrimitiveBehaviors::UnlimitedNaturalFunctions::Max",
		"PrimitiveBehaviors::UnlimitedNaturalFunctions::Min",
	}
	if strings.Join(gaps, "\n") != strings.Join(want, "\n") {
		t.Errorf("library gaps = %v, want %v", gaps, want)
	}
	if len(called) != 46 {
		t.Errorf("%d library behaviors called, want 46", len(called))
	}
	for name := range libraryCounterparts {
		if !called[name] {
			t.Errorf("counterpart for %s, which no activity calls", name)
		}
	}
}

const refiringModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Refiring">
  <packagedElement xmi:type="uml:Activity" xmi:id="a" name="Outer">
    <node xmi:type="uml:CallBehaviorAction" xmi:id="callInner" name="Call(Inner)" behavior="b"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v1" name="Value(1)">
      <result xmi:type="uml:OutputPin" xmi:id="v1r" name="result"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v2" name="Value(2)">
      <result xmi:type="uml:OutputPin" xmi:id="v2r" name="result"/>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="fed" name="Fed" behavior="b">
      <argument xmi:type="uml:InputPin" xmi:id="fedIn" name="in"/>
    </node>
    <node xmi:type="uml:MergeNode" xmi:id="merge" name="Merge"/>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="merged" name="Merged" behavior="b"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f1" source="v1r" target="fedIn"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f2" source="v2r" target="fedIn"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c1" source="v1" target="merge"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c2" source="v2" target="merge"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c3" source="merge" target="merged"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="b" name="Inner">
    <node xmi:type="uml:CallBehaviorAction" xmi:id="callOuter" name="Call(Outer)" behavior="a"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="iv" name="Value(0)">
      <result xmi:type="uml:OutputPin" xmi:id="ivr" name="result"/>
    </node>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="c" name="Destroyer">
    <node xmi:type="uml:DestroyObjectAction" xmi:id="destroy" name="Destroy">
      <target xmi:type="uml:InputPin" xmi:id="dt" name="target"/>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="callOuter2" name="Call(Outer)" behavior="a"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="d" name="Starter">
    <node xmi:type="uml:CallBehaviorAction" xmi:id="callDestroyer" name="Call(Destroyer)" behavior="c"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="worker" name="Worker" isActive="true" classifierBehavior="workerRun">
    <ownedBehavior xmi:type="uml:Activity" xmi:id="workerRun" name="Run">
      <node xmi:type="uml:DestroyObjectAction" xmi:id="destroySelf" name="Destroy">
        <target xmi:type="uml:InputPin" xmi:id="dst" name="target"/>
      </node>
    </ownedBehavior>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="e" name="Creator">
    <node xmi:type="uml:CreateObjectAction" xmi:id="createWorker" name="Create(Worker)" classifier="worker">
      <result xmi:type="uml:OutputPin" xmi:id="cwr" name="result" type="worker"/>
    </node>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="f" name="Launcher">
    <node xmi:type="uml:CreateObjectAction" xmi:id="createWorker2" name="Create(Worker)" classifier="worker">
      <result xmi:type="uml:OutputPin" xmi:id="cwr2" name="result"/>
    </node>
    <node xmi:type="uml:ForkNode" xmi:id="workerFork" name="Fork"/>
    <node xmi:type="uml:StartObjectBehaviorAction" xmi:id="startWorker" name="Start(Worker)">
      <object xmi:type="uml:InputPin" xmi:id="swo" name="object"/>
    </node>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f3" source="cwr2" target="workerFork"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f4" source="workerFork" target="swo"/>
  </packagedElement>
</uml:Model>
`

// fires is one execution of activity firing action n times, with the ids the
// driver would resolve from m; an unknown activity or action carries none.
func fires(m *Model, activity, action string, n int) []ExpectedEvent {
	var activityID, actionID string
	if a := m.ActivityNamed(activity); a != nil {
		activityID = a.ID
		if node := a.NodeNamed(action); node != nil {
			actionID = node.ID
		}
	}
	events := []ExpectedEvent{{Kind: "Execute", Activity: activity, ID: activityID}}
	for i := 0; i < n; i++ {
		events = append(events, ExpectedEvent{Kind: "Fire", Activity: activity, Action: action, ID: actionID})
	}
	return append(events, ExpectedEvent{Kind: "Complete", Activity: activity, ID: activityID})
}

// TestClassifyFilesReFiringByItsCause files an action the trace fired twice
// as differing by design only when an object flow feeds it; a merge re-fires
// in SysML v2 too. Mutually recursive activities classify without looping,
// and a dependency's class propagates to what calls it.
func TestClassifyFilesReFiringByItsCause(t *testing.T) {
	m, err := ReadModel(strings.NewReader(refiringModel), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", m.Diagnostics)
	}
	outer := activity(t, m, "Outer")

	if c := Classify(outer, nil); c.Class != Expressible {
		t.Errorf("without a trace: %s: %s", c.Class, c.Reason())
	}
	if c := Classify(outer, &ExpectedActivity{ID: outer.ID, Name: outer.Name, Events: fires(m, "Outer", "Merged", 2)}); c.Class != Expressible {
		t.Errorf("merge-fed re-firing: %s: %s", c.Class, c.Reason())
	}
	c := Classify(outer, &ExpectedActivity{ID: outer.ID, Name: outer.Name, Events: fires(m, "Outer", "Fed", 2)})
	if c.Class != DiffersByDesign || !strings.HasSuffix(c.Reason(), ": Outer.Fed fired 2 times") {
		t.Errorf("object-fed re-firing: %s: %s", c.Class, c.Reason())
	}
	// A called activity's action re-firing counts against the caller, resolved
	// through the model; an action the model lacks cannot be object-fed.
	nested := fires(m, "Outer", "Call(Inner)", 1)
	nested = append(nested[:len(nested)-1], append(fires(m, "Inner", "Value(0)", 2), nested[len(nested)-1])...)
	if c := Classify(outer, &ExpectedActivity{ID: outer.ID, Name: outer.Name, Events: nested}); c.Class != Expressible {
		t.Errorf("control-only nested re-firing: %s: %s", c.Class, c.Reason())
	}
	if c := Classify(outer, &ExpectedActivity{ID: outer.ID, Name: outer.Name, Events: fires(m, "Nowhere", "Fed", 2)}); c.Class != Expressible {
		t.Errorf("re-firing in an unknown activity: %s: %s", c.Class, c.Reason())
	}
	// Two executions in one record fire the action once each: no re-firing.
	twice := append(fires(m, "Outer", "Fed", 1), fires(m, "Outer", "Fed", 1)...)
	if c := Classify(outer, &ExpectedActivity{ID: outer.ID, Name: outer.Name, Events: twice}); c.Class != Expressible {
		t.Errorf("two executions: %s: %s", c.Class, c.Reason())
	}

	starter := Classify(activity(t, m, "Starter"), nil)
	want := "dependency on a not-expressible behavior (a behavior it calls or starts is itself not expressible): Destroyer (DestroyObjectAction)"
	if starter.Class != NotExpressible || starter.Reason() != want {
		t.Errorf("Starter = %s: %s", starter.Class, starter.Reason())
	}
	if c, ok := starter.Class.Bucket(); !ok || c != BucketNotExpressible {
		t.Errorf("Bucket() = %s, %v", c, ok)
	}
	if _, ok := Expressible.Bucket(); ok {
		t.Error("an expressible class has a bucket before a run")
	}

	// Creating an object runs nothing of its class; starting it runs the
	// classifier behavior, whose class then propagates.
	if c := Classify(activity(t, m, "Creator"), nil); c.Class != Expressible {
		t.Errorf("Creator = %s: %s", c.Class, c.Reason())
	}
	launcher := Classify(activity(t, m, "Launcher"), nil)
	want = "dependency on a not-expressible behavior (a behavior it calls or starts is itself not expressible): Run (DestroyObjectAction)"
	if launcher.Class != NotExpressible || launcher.Reason() != want {
		t.Errorf("Launcher = %s: %s", launcher.Class, launcher.Reason())
	}
}

func TestExpressibilityStringOutsideTheClasses(t *testing.T) {
	for _, c := range []Expressibility{-1, Expressibility(len(expressibilityNames))} {
		if got, want := c.String(), fmt.Sprintf("Expressibility(%d)", int(c)); got != want {
			t.Errorf("%d.String() = %q, want %q", int(c), got, want)
		}
	}
}
