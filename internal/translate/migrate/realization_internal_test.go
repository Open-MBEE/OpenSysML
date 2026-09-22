package migrate

import (
	"strings"
	"testing"
)

// realizationModel wraps classifiers realizing the interface Link (_link);
// stereotype is the application the classifier _c carries, if any.
func realizationModel(members, stereotype string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Interface" xmi:id="_link" name="Link"/>
    ` + members + `
  </uml:Model>
  ` + stereotype + `
</xmi:XMI>`
}

func TestInterfaceRealizationForms(t *testing.T) {
	for _, tc := range []struct {
		name, members, stereotype string
		want                      []string
		verdict                   Verdict
	}{
		{"a class realizing a port def carries it as a port",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Node">
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_link" contract="_link"/>
			 </packagedElement>`, "",
			[]string{"part def Node {\n    port link : Link;\n}"}, Approximated},
		{"the port name steps aside for a member of the same name",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Node">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="link"/>
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_link" contract="_link"/>
			 </packagedElement>`, "",
			[]string{"port 'link 2' : Link;"}, Approximated},
		{"a port already typed by the interface carries the realization",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Node">
			   <ownedAttribute xmi:type="uml:Port" xmi:id="_p" name="up" type="_link"/>
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_link" contract="_link"/>
			 </packagedElement>`, "",
			[]string{"part def Node {\n    port up : Link;\n}"}, Approximated},
		{"the supplier stands in for a missing contract",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Node">
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_link"/>
			 </packagedElement>`, "",
			[]string{"port link : Link;"}, Approximated},
		{"an interface realizing an interface specializes it once",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="FastLink">
			   <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_link"/>
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_link" contract="_link"/>
			 </packagedElement>`, `<sysml:InterfaceBlock xmi:id="_s" base_Class="_c"/>`,
			[]string{"port def FastLink :> Link;"}, Mapped},
		{"a definition of another kind is refused",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Rule">
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_link" contract="_link"/>
			 </packagedElement>`, `<sysml:ConstraintBlock xmi:id="_s" base_Class="_c"/>`,
			[]string{"not migrated: InterfaceRealization (_ir) — a constraint def cannot specialize the port def the interface Link becomes"}, Unmapped},
		{"an interface outside the document is refused",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Node">
			   <interfaceRealization xmi:type="uml:InterfaceRealization" xmi:id="_ir" client="_c" supplier="_gone" contract="_gone"/>
			 </packagedElement>`, "",
			[]string{"the realized interface is not in the document; 1 contract reference(s) resolve to nothing in the document (_gone)"}, Unmapped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("realized.xmi", []byte(realizationModel(tc.members, tc.stereotype)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			var found bool
			for _, e := range r.Report.Entries {
				if e.ID != "_ir" {
					continue
				}
				found = true
				if e.Verdict != tc.verdict {
					t.Errorf("verdict %s, want %s (%s)", e.Verdict, tc.verdict, e.Note)
				}
			}
			if !found {
				t.Error("the realization is missing from the report")
			}
		})
	}
}
