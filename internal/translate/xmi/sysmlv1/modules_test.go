package sysmlv1

import (
	"strings"
	"testing"
)

const snapshotDocument = `<?xml version="1.0" encoding="ASCII"?>
<xmi:XMI xmi:version="2.0" xmlns:xmi="http://www.omg.org/XMI" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:uml="http://www.nomagic.com/magicdraw/UML/2.5.1.1">
  <uml:Package xmi:id="_root" name="SysML Profile">
    <packagedElement xsi:type="uml:Profile" xmi:id="_sysml" name="SysML">
      <packagedElement xsi:type="uml:Stereotype" xmi:id="_block" name="Block"/>
      <packagedElement xsi:type="uml:Package" xmi:id="_ext" name="Non-Normative Extensions">
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_system" name="System">
          <generalization xmi:id="_system_gen" general="_block"/>
        </packagedElement>
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_subsystem" name="Subsystem">
          <generalization xmi:id="_subsystem_gen">
            <general xsi:type="uml:Stereotype" href="#_system"/>
          </generalization>
        </packagedElement>
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_domain" name="Domain"/>
      </packagedElement>
    </packagedElement>
    <packagedElement xsi:type="uml:Profile" xmi:id="_org" name="Org">
      <packagedElement xsi:type="uml:Stereotype" xmi:id="_org_subsystem" name="Subsystem"/>
      <packagedElement xsi:type="uml:Stereotype" xmi:id="_org_structural" name="Structural">
        <generalization xmi:id="_org_structural_gen">
          <general xsi:type="uml:Stereotype" href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#Block"/>
        </generalization>
      </packagedElement>
    </packagedElement>
  </uml:Package>
</xmi:XMI>`

func snapshotModel(t *testing.T, applications string) *Model {
	t.Helper()
	model := `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML" xmlns:Org="http://www.magicdraw.com/schemas/Org.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_d" name="D"/>
  </uml:Model>
` + applications + `
</xmi:XMI>`
	data := archive(t, "", map[string][]byte{
		"com.nomagic.ci.metamodel.project":      []byte(`<?xml version="1.0"?><project/>`),
		"com.nomagic.magicdraw.uml_model.model": []byte(model),
		"proxy.local__PROJECT$h1_resource_com$dnomagic$dmagicdraw$duml_umodel$dshared_umodel$dsnapshot": []byte(snapshotDocument),
	})
	m, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func generalNames(s *Stereotype) string {
	var names []string
	for _, g := range s.Generals {
		names = append(names, g.Parent.Name+"::"+g.Name)
	}
	return strings.Join(names, ", ")
}

// An application of a stereotype only a module snapshot declares inherits the
// snapshot's generalizations, through a profile its namespace denotes by name.
func TestModuleSnapshotGenerals(t *testing.T) {
	m := snapshotModel(t, `  <sysml:Subsystem xmi:id="_a1" base_Class="_c"/>
  <Org:Subsystem xmi:id="_a2" base_Class="_d"/>
  <sysml:Domain xmi:id="_a3" base_Class="_d"/>
  <Org:Structural xmi:id="_a4" base_Class="_d"/>`)
	s := m.Lookup("_c").Stereotype("Subsystem")
	if s.Definition != nil || !s.Module {
		t.Errorf("a snapshot declaration became the Definition %+v, Module %v", s.Definition, s.Module)
	}
	if d := m.Lookup("_d").Stereotype("Domain"); !d.Module || len(d.Generals) != 0 {
		t.Errorf("a snapshot declaration with no generalization: Module %v, generals %q", d.Module, generalNames(d))
	}
	if st := m.Lookup("_d").Stereotype("Structural"); len(st.Generals) != 1 || !st.Generals[0].IsProxy() || st.Generals[0].Name != "Block" || st.Generals[0].Href != "http://www.omg.org/spec/SysML/20181001/SysML.xmi#Block" {
		t.Errorf("an href into the OMG profile: generals %+v", st.Generals)
	}
	if got := generalNames(s); got != "SysML::System, SysML::Block" {
		t.Errorf("Subsystem generals = %q, want SysML::System, SysML::Block", got)
	}
	if s.Generals[0].Type != "Stereotype" || s.Generals[0].Parent.Type != "Profile" || s.Generals[0].IsProxy() {
		t.Errorf("snapshot general %+v is not a stereotype of a profile", s.Generals[0])
	}
	if got := generalNames(m.Lookup("_d").Stereotype("Subsystem")); got != "" {
		t.Errorf("Org's own Subsystem inherited %q", got)
	}
	if ref := m.StereotypeRef("_system"); ref.Name != "System" || !IsSysMLNamespace(ref.Namespace) {
		t.Errorf("StereotypeRef(_system) = %+v, want System in the SysML namespace", ref)
	}
	var names []string
	ancestors, declared := m.StereotypeAncestors("local:/PROJECT-1?resource=com.nomagic.magicdraw.uml_umodel.shared_umodel#_subsystem")
	for _, a := range ancestors {
		names = append(names, a.Name)
	}
	if got := strings.Join(names, ", "); got != "System, Block" || !declared {
		t.Errorf("StereotypeAncestors(Subsystem) = %q, %v; want System, Block, true", got, declared)
	}
	if ancestors, declared := m.StereotypeAncestors("_domain"); len(ancestors) != 0 || !declared {
		t.Errorf("StereotypeAncestors(Domain) = %v, %v; want none, true", ancestors, declared)
	}
	if _, declared := m.StereotypeAncestors("_nowhere"); declared {
		t.Error("an id no snapshot declares is declared")
	}
}

// The tool's stereotype table picks the declaration when two profiles share a name.
func TestModuleSnapshotGeneralsByHrefTable(t *testing.T) {
	m := snapshotModel(t, `  <sysml:Subsystem xmi:id="_a1" base_Class="_c"/>
  <xmi:Extension extender="MagicDraw UML 2022x">
    <stereotypesHREFS>
      <stereotype name="sysml:Subsystem" stereotypeHREF="local:/PROJECT-1?resource=com.nomagic.magicdraw.uml_umodel.shared_umodel#_org_subsystem"/>
    </stereotypesHREFS>
  </xmi:Extension>`)
	if got := generalNames(m.Lookup("_c").Stereotype("Subsystem")); got != "" {
		t.Errorf("the table's Org::Subsystem inherited %q", got)
	}
}
