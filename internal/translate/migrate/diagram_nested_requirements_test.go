package migrate

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestUseCaseDiagramExposesNestedRequirementsByQualifiedName(t *testing.T) {
	members := `<packagedElement xmi:type="uml:Package" xmi:id="_reqs" name="Requirements">
  <packagedElement xmi:type="uml:Class" xmi:id="_root" name="Root">
    <nestedClassifier xmi:type="uml:Class" xmi:id="_child" name="Child"/>
  </packagedElement>
</packagedElement>
<packagedElement xmi:type="uml:UseCase" xmi:id="_use" name="Measure"/>
<packagedElement xmi:type="uml:Abstraction" xmi:id="_refine" client="_use" supplier="_child"/>`
	data := diagramModel(members, diagram("_d", "Cases", "_sys", "SysML Use Case Diagram", "_use", "_child", "_refine"),
		`<sysml:Requirement xmi:id="_sr" base_Class="_root"/>`,
		`<sysml:Requirement xmi:id="_sc" base_Class="_child"/>`,
		`<sysml:Refine xmi:id="_sf" base_Abstraction="_refine"/>`)
	r, err := Migrate("nested.xmi", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Notation), "expose Requirements::Root::Child;") {
		t.Fatalf("missing qualified nested requirement exposure:\n%s", r.Notation)
	}
	p := parser.New(source.New("nested.sysml", r.Notation))
	p.ParseFile()
	if ds := p.Diagnostics; len(ds) != 0 {
		t.Fatalf("generated notation has diagnostics: %v\n%s", ds, r.Notation)
	}
}
