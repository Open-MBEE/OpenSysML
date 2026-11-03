package migrate_test

import (
	"strings"
	"testing"
)

const bufferedCallMachine = `
    <packagedElement xmi:type="uml:Package" xmi:id="_tables" name="Tables">
      <packagedElement xmi:type="uml:Class" xmi:id="_vp" name="Figures Viewpoint" classifierBehavior="_method">
        <ownedBehavior xmi:type="uml:Activity" xmi:id="_method" name="Figures Method">
          <node xmi:type="uml:InitialNode" xmi:id="_mi"/>
          <node xmi:type="uml:CallBehaviorAction" xmi:id="_image" name="Image"/>
          <edge xmi:type="uml:ControlFlow" xmi:id="_mf" source="_mi" target="_image"/>
        </ownedBehavior>
      </packagedElement>
      <packagedElement xmi:type="uml:Class" xmi:id="_report" name="Report">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_report_calls" name="calls" type="_calls_view" aggregation="composite"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Class" xmi:id="_calls_view" name="Calls">
        <generalization xmi:type="uml:Generalization" xmi:id="_conform" general="_vp"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Dependency" xmi:id="_expose" client="_calls_view" supplier="_listed_diagram"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="Pkg">
      <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
      <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
      <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
      <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
      <packagedElement xmi:type="uml:Activity" xmi:id="_guide" name="Guide"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_exec" name="Executive" classifierBehavior="_sm">
        <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Behavior">
          <region xmi:type="uml:Region" xmi:id="_r">
            <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
            <subvertex xmi:type="uml:State" xmi:id="_acq" name="Acquiring">
              <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dPing" event="_pingEv"/>
              <doActivity xmi:type="uml:Activity" xmi:id="_acquire" name="Acquire">
                <node xmi:type="uml:InitialNode" xmi:id="_ai"/>
                <node xmi:type="uml:CallBehaviorAction" xmi:id="_call" name="call" behavior="_guide"/>
                <edge xmi:type="uml:ControlFlow" xmi:id="_af" source="_ai" target="_call"/>
              </doActivity>
              <exit xmi:type="uml:Activity" xmi:id="_release" name="Release">
                <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
                <node xmi:type="uml:CallBehaviorAction" xmi:id="_free" name="free" behavior="_guide"/>
                <edge xmi:type="uml:ControlFlow" xmi:id="_rf" source="_ri" target="_free"/>
              </exit>
            </subvertex>
            <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
            <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_acq"/>
            <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_acq" target="_idle">
              <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
            </transition>
          </region>
        </ownedBehavior>
      </packagedElement>
    </packagedElement>`

const bufferedCallTables = `
  <sysml:Block xmi:id="_execBlock" base_Class="_exec"/>
  <sysml:Viewpoint xmi:id="_st_vp" base_Class="_vp"/>
  <sysml:View xmi:id="_st_calls_view" base_Class="_calls_view"/>
  <sysml:Conform xmi:id="_st_conform" base_Generalization="_conform"/>
  <sysml:Expose xmi:id="_st_expose" base_Dependency="_expose"/>
  <Document_Profile_:Document xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi" xmi:id="_st_report" base_Class="_report"/>
  <Document_Profile_:Image xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi" xmi:id="_st_image" base_CallBehaviorAction="_image"/>
  <xmi:Extension extender="Example UML Tool 1.0">
    <modelExtension>
      <ownedDiagram xmi:type="uml:Diagram" xmi:id="_listed_diagram" name="Listed Calls" ownerOfDiagram="_tables">
        <xmi:Extension extender="Example UML Tool 1.0">
          <diagramRepresentation>
            <diagram:DiagramRepresentationObject xmlns:diagram="http://www.example.com/tool/diagram" type="Generic Table" umlType="Class Diagram">
              <diagramContents/>
            </diagram:DiagramRepresentationObject>
          </diagramRepresentation>
        </xmi:Extension>
      </ownedDiagram>
    </modelExtension>
  </xmi:Extension>
  <MagicDraw_Profile:DiagramTable xmlns:MagicDraw_Profile="http://www.omg.org/spec/UML/20131001/MagicDrawProfile" xmi:id="_listed_table" base_Diagram="_listed_diagram">
    <rowElementType href="http://www.omg.org/spec/UML/20131001/UML.xmi#CallBehaviorAction"/>
    <rowElements xmi:idref="_call"/>
    <rowElements xmi:idref="_free"/>
    <columnIds>QPROP:Element:qualifiedName</columnIds>
  </MagicDraw_Profile:DiagramTable>
`

// A state that defers a signal writes its do behavior inside the encoding's
// buffer action and its exit behavior inside the flush action, so a table a
// document plans before the state is written names their actions through those.
func TestTableQueriesNameActionsInADeferralBuffer(t *testing.T) {
	r := migrateDocument(t, bufferedCallMachine, bufferedCallTables)
	wantClean(t, "buffered_call_tables.sysml", r)
	s := session(t, r)
	got := rows(t, s, "Model::Tables::'Listed Calls Rows'")
	for _, action := range []string{"Acquiring::buffer::Acquire::call", "Acquiring::flush::Release::free"} {
		if !strings.Contains(got, "Executive::Behavior::"+action) {
			t.Errorf("Listed Calls rows lack %s:\n%s", action, got)
		}
	}
	for _, document := range []string{"Model::Tables::'Listed Calls Document'", "Model::Tables::'Report Document'"} {
		page := html(t, s, document)
		if !strings.Contains(page, "Acquiring::buffer::Acquire::call") {
			t.Errorf("%s does not render the buffered actions:\n%s", document, page)
		}
	}
}

// A signal the do behavior itself declares is named while that behavior's
// state plans its deferrals, which must not plan them again.
func TestDeferredSignalDeclaredInTheBufferedBehavior(t *testing.T) {
	machine := strings.Replace(bufferedCallMachine, `<packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>`, "", 1)
	machine = strings.Replace(machine, `<node xmi:type="uml:InitialNode" xmi:id="_ai"/>`,
		`<nestedClassifier xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
                <node xmi:type="uml:InitialNode" xmi:id="_ai"/>`, 1)
	r := migrateDocument(t, machine, bufferedCallTables)
	wantClean(t, "buffered_signal_tables.sysml", r)
	got := rows(t, session(t, r), "Model::Tables::'Listed Calls Rows'")
	if !strings.Contains(got, "Executive::Behavior::Acquiring::buffer::Acquire::call") {
		t.Errorf("Listed Calls rows lack the buffered call:\n%s", got)
	}
}
