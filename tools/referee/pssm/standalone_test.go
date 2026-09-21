package pssm

import (
	"strings"
	"testing"

	oreport "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// standaloneSuite is one test whose target is a state machine of its own, with
// attribute balance (constructor writes 15), operation bump (adds 100) and a region.
func standaloneSuite(expected string) string {
	return fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:Package" xmi:id="areaSA" name="Standalone">
` + registration("Standalone", "semSA", "Standalone 001", expected) +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgSA" name="001">
    <packagedElement xmi:type="uml:Class" xmi:id="semSA" name="SA_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semSAGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:StateMachine" xmi:id="smSA" name="SA_Test">
      <generalization xmi:type="uml:Generalization" xmi:id="smSAGen" general="clsTarget"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="saBalance" name="balance">
        <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="opBump" name="bump" method="saBumpMethod"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="saBumpMethod" name="bump$method$1" specification="opBump">
        <node xmi:type="uml:ReadSelfAction" xmi:id="bSelf"><result xmi:type="uml:OutputPin" xmi:id="bSelfOut"/></node>
        <node xmi:type="uml:ForkNode" xmi:id="bFork"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="bRead" structuralFeature="saBalance">
          <object xmi:type="uml:InputPin" xmi:id="bReadObj"/>
          <result xmi:type="uml:OutputPin" xmi:id="bReadOut"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="bVal">
          <result xmi:type="uml:OutputPin" xmi:id="bValOut"/>
          <value xmi:type="uml:LiteralInteger" xmi:id="bLit" value="100"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="bPlus" name="Call('+')">
          <behavior href="http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-plus"/>
          <argument xmi:type="uml:InputPin" xmi:id="bPlusA"/>
          <argument xmi:type="uml:InputPin" xmi:id="bPlusB"/>
          <result xmi:type="uml:OutputPin" xmi:id="bPlusOut"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="bWrite" structuralFeature="saBalance" isReplaceAll="true">
          <object xmi:type="uml:InputPin" xmi:id="bWriteObj"/>
          <value xmi:type="uml:InputPin" xmi:id="bWriteVal"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="bE1" source="bSelfOut" target="bFork"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="bE2" source="bFork" target="bReadObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="bE3" source="bFork" target="bWriteObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="bE4" source="bReadOut" target="bPlusA"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="bE5" source="bValOut" target="bPlusB"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="bE6" source="bPlusOut" target="bWriteVal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="saFactory" name="SA_Test$factory">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="saFactoryRet" direction="return" type="smSA"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="saFactoryOut" name="Return" parameter="saFactoryRet"/>
        <node xmi:type="uml:CreateObjectAction" xmi:id="fCreate" name="Create" classifier="smSA">
          <result xmi:type="uml:OutputPin" xmi:id="fCreateOut"/>
        </node>
        <node xmi:type="uml:StartObjectBehaviorAction" xmi:id="fStart" name="Start">
          <object xmi:type="uml:InputPin" xmi:id="fStartObj"/>
        </node>
        <node xmi:type="uml:ForkNode" xmi:id="fFork" name="Fork(t)"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE1" source="fCreateOut" target="fFork"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE2" source="fFork" target="fStartObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fFork" target="saFactoryOut"/>
        ` + factoryWrite("saBalance", "uml:LiteralInteger", "15") + `
      </ownedBehavior>
      <region xmi:type="uml:Region" xmi:id="regSA" name="Region1">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="saInit" name="Initial1"/>
        <subvertex xmi:type="uml:State" xmi:id="saWait" name="wait"/>
        <subvertex xmi:type="uml:State" xmi:id="saS1" name="S1">
          ` + traceCall("entry", "saS1entry", "S1(entry)") + `
        </subvertex>
        <subvertex xmi:type="uml:FinalState" xmi:id="saFin" name="FinalState1"/>
        <transition xmi:type="uml:Transition" xmi:id="saT1" name="T1" source="saInit" target="saWait"/>
        <transition xmi:type="uml:Transition" xmi:id="saT2" name="T2" source="saWait" target="saS1">
          <trigger xmi:type="uml:Trigger" xmi:id="saT2trig" event="evStart"/>
          <effect xmi:type="uml:Activity" xmi:id="saT2effect" name="effect">
            <node xmi:type="uml:ReadSelfAction" xmi:id="eSelf"><result xmi:type="uml:OutputPin" xmi:id="eSelfOut"/></node>
            <node xmi:type="uml:CallOperationAction" xmi:id="eBump" operation="opBump">
              <target xmi:type="uml:InputPin" xmi:id="eBumpTarget"/>
            </node>
            <node xmi:type="uml:ReadSelfAction" xmi:id="eSelf2"><result xmi:type="uml:OutputPin" xmi:id="eSelf2Out"/></node>
            <node xmi:type="uml:ValueSpecificationAction" xmi:id="eVal">
              <result xmi:type="uml:OutputPin" xmi:id="eValOut"/>
              <value xmi:type="uml:LiteralString" xmi:id="eLit" value="T2(effect)"/>
            </node>
            <node xmi:type="uml:CallOperationAction" xmi:id="eTrace" operation="opTrace">
              <target xmi:type="uml:InputPin" xmi:id="eTraceTarget"/>
              <argument xmi:type="uml:InputPin" xmi:id="eTraceArg"/>
            </node>
            <edge xmi:type="uml:ObjectFlow" xmi:id="eE1" source="eSelfOut" target="eBumpTarget"/>
            <edge xmi:type="uml:ObjectFlow" xmi:id="eE2" source="eSelf2Out" target="eTraceTarget"/>
            <edge xmi:type="uml:ObjectFlow" xmi:id="eE3" source="eValOut" target="eTraceArg"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="eE4" source="eBump" target="eTrace"/>
          </effect>
        </transition>
        <transition xmi:type="uml:Transition" xmi:id="saT3" name="T3" source="saS1" target="saFin">
          <trigger xmi:type="uml:Trigger" xmi:id="saT3trig" event="evContinue"/>
        </transition>
      </region>
    </packagedElement>
` + tester("SA_Tester", "sigContinue") + `  </packagedElement>
  </packagedElement>
` + fixtureTail
}

// A standalone state machine is read as the target itself, with its attributes,
// operations, methods and constructor, and has no owning class.
func TestReadStandaloneMachine(t *testing.T) {
	s := readFixture(t, standaloneSuite("T2(effect)::S1(entry)"))
	noDiagnostics(t, s)
	tt := s.Tests[0]
	if tt.Target == nil || !tt.Target.Standalone || tt.Machine == nil || tt.Machine.Name != "SA_Test" || tt.Machine.Owner != "" {
		t.Fatalf("standalone machine misread: %+v", tt.Target)
	}
	if len(tt.Target.Attributes) != 1 || tt.Target.Attributes[0].Name != "balance" || tt.Target.Attributes[0].Type != "Integer" {
		t.Errorf("attributes = %+v", tt.Target.Attributes)
	}
	op := tt.Target.Operation("opBump")
	if op == nil || op.Method == nil || op.Method.Body == nil || len(op.Method.Body.Statements) != 1 ||
		op.Method.Body.Statements[0].String() != "this.balance := plus(this.balance, 100)" {
		t.Errorf("operation bump = %+v", op)
	}
	var factory *Behavior
	for _, b := range tt.Target.Behaviors {
		if b.Name == "SA_Test$factory" {
			factory = b
		}
	}
	if factory == nil || factory.Body == nil || len(factory.Body.Statements) != 3 {
		t.Errorf("factory = %+v", factory)
	}
	if len(tt.Stimulation.Statements) != 2 || tt.Stimulation.Statements[0].Kind != StmtAccept {
		t.Errorf("stimulation = %+v", tt.Stimulation)
	}
}

// A standalone machine is a standard test whose constructor write, method
// call and traces translate and run.
func TestStandaloneMachineTranslatesAndRuns(t *testing.T) {
	s := readFixture(t, standaloneSuite("T2(effect)::S1(entry)"))
	noDiagnostics(t, s)
	tt := s.Tests[0]
	if c := Classify(tt); c.Class != Standard || len(c.Uses) != 0 {
		t.Fatalf("classified %s %v", c.Class, c.Uses)
	}
	m, err := Emit(s, tt)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"attribute balance : Integer = 15;", "assign balance := (balance + 100);"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	report, err := Referee(t.Context(), s, Provenance{Document: "fixture", Tests: 1}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	row := report.Tests[0]
	if row.Bucket != oreport.BucketPass || len(row.Reasons) != 0 {
		t.Fatalf("bucket %s, reasons %q; want pass", row.Bucket, row.Reasons)
	}
	if strings.Join(row.Reached, ",") != "T2(effect)::S1(entry)" {
		t.Errorf("reached %q", row.Reached)
	}
}
