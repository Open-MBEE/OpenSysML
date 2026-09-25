package migrate

import (
	"strings"
	"testing"
)

// TestSynthesizedNamesMarked covers declarations the migrator names itself: each
// written one is listed by the SynthesizedName marker of the body holding it.
func TestSynthesizedNamesMarked(t *testing.T) {
	for _, tc := range []struct {
		name, members string
		wantNotation  []string
	}{
		{"an anonymous parameter",
			`<packagedElement xmi:type="uml:Activity" xmi:id="_b" name="Run">
			   <ownedParameter xmi:type="uml:Parameter" xmi:id="_p" direction="in">
			     <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
			   </ownedParameter>
			 </packagedElement>`,
			[]string{"action def Run {\n    in real : ScalarValues::Real;\n    metadata MigrationMetadata::SynthesizedName about real;\n}"}},
		{"an anonymous port's payload",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Ctrl">
			   <ownedAttribute xmi:type="uml:Port" xmi:id="_q" type="_pump" aggregation="composite"/>
			 </packagedElement>`,
			[]string{"port {\n        ref item pump : Sys::Pump;\n        metadata MigrationMetadata::SynthesizedName about pump;\n    }\n}"}},
		{"an include named after its addition",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Drive">
			   <include xmi:type="uml:Include" xmi:id="_inc" addition="_start"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:UseCase" xmi:id="_start" name="Start"/>`,
			[]string{"use case def Drive {\n    include use case start : Start;\n    metadata MigrationMetadata::SynthesizedName about start;\n}"}},
		{"an instance's slot of an anonymous feature",
			`<packagedElement xmi:type="uml:Class" xmi:id="_tank" name="Tank">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_f">
			     <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
			   </ownedAttribute>
			 </packagedElement>
			 <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_t1" name="t1" classifier="_tank">
			   <slot xmi:type="uml:Slot" xmi:id="_s" definingFeature="_f">
			     <value xmi:type="uml:LiteralReal" xmi:id="_v" value="2.5"/>
			   </slot>
			 </packagedElement>`,
			[]string{"attribute :>> real = 2.5;\n    metadata MigrationMetadata::SynthesizedName about real;"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("names.xmi", []byte(diagramModel(tc.members, "", `<sysml:Block xmi:id="_sb2" base_Class="_tank"/>`)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.wantNotation {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			if strings.Contains(got, "about ''") {
				t.Errorf("an anonymous declaration is marked:\n%s", got)
			}
		})
	}
}
