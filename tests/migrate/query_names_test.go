package migrate_test

import (
	"strings"
	"testing"
)

func TestTableQueriesQuoteTagPathSegments(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Profile" xmi:id="_profile" name="Review Profile" URI="http://example.com/schemas/Review_Profile.xmi">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_reviewed" name="Reviewed">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_reviewed_base" name="base_Class" association="_extension">
          <type xmi:type="uml:Class" href="http://www.omg.org/spec/UML/20131001/UML.xmi#Class"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_reviewed_level" name="level">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_reviewed_scope" name="Safety::Scope">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>
        </ownedAttribute>
      </packagedElement>
      <packagedElement xmi:type="uml:Extension" xmi:id="_extension" memberEnd="_reviewed_base _extension_end">
        <ownedEnd xmi:type="uml:ExtensionEnd" xmi:id="_extension_end" name="extension_Reviewed" type="_reviewed" aggregation="composite"/>
      </packagedElement>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_parts" name="Plumbing">
      <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_tank" name="Tank"/>
    </packagedElement>`, `
  <xmi:Extension extender="Example UML Tool 1.0">
    <modelExtension>
      <ownedDiagram xmi:type="uml:Diagram" xmi:id="_parts_diagram" name="Reviewed Parts" ownerOfDiagram="_parts">
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
  <sysml:Block xmi:id="_s_pump" base_Class="_pump"/>
  <sysml:Block xmi:id="_s_valve" base_Class="_valve"/>
  <sysml:Block xmi:id="_s_tank" base_Class="_tank"/>
  <Review_Profile:Reviewed xmlns:Review_Profile="http://example.com/schemas/Review_Profile.xmi" xmi:id="_app_pump" base_Class="_pump" level="1"/>
  <Review_Profile:Reviewed xmlns:Review_Profile="http://example.com/schemas/Review_Profile.xmi" xmi:id="_app_valve" base_Class="_valve" level="3"/>
  <Review_Profile:Reviewed xmlns:Review_Profile="http://example.com/schemas/Review_Profile.xmi" xmi:id="_app_tank" base_Class="_tank" level="2"/>
  <MagicDraw_Profile:DiagramTable xmlns:MagicDraw_Profile="http://www.omg.org/spec/UML/20131001/MagicDrawProfile" xmi:id="_parts_table" base_Diagram="_parts_diagram" scope="_parts">
    <rowElementType href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML.Block"/>
    <columnIds>QPROP:Element:name</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Review Profile::Reviewed&gt;&gt;.level</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Review Profile::Reviewed&gt;&gt;.Safety::Scope</columnIds>
    <sort>QPROP:stereotypeTags:&lt;&lt;Review Profile::Reviewed&gt;&gt;.Safety::Scope^Asc</sort>
    <sort>QPROP:stereotypeTags:&lt;&lt;Review Profile::Reviewed&gt;&gt;.level^Desc</sort>
  </MagicDraw_Profile:DiagramTable>`)
	wantClean(t, "quoted_tag_paths.sysml", r)
	notation := string(r.Notation)
	for _, spelled := range []string{
		`property = "Model::'Review Profile'::Reviewed::level"`,
		`property = "Model::'Review Profile'::Reviewed::'Safety::Scope'"`,
		`DocumentQueries::Column(name = "Safety::Scope", cell = { in row : 'Review Profile'::Reviewed; row.'Safety::Scope' ?? "" })`,
	} {
		if !strings.Contains(notation, spelled) {
			t.Errorf("the table query does not spell %s:\n%s", spelled, notation)
		}
	}
	s := session(t, r)
	wantInOrder(t, "Reviewed Parts rows", rows(t, s, "Model::Plumbing::'Reviewed Parts Rows'"),
		"Columns: name, level, Safety::Scope",
		"Model::Plumbing::Valve", "level = 3",
		"Model::Plumbing::Tank", "level = 2",
		"Model::Plumbing::Pump", "level = 1")
}
