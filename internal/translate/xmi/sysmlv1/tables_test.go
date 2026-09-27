package sysmlv1

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// hexBytes encodes s the way MagicDraw serializes a diagram's binary
// properties: one hexadecimal byte per whitespace-separated token.
func hexBytes(s string) string {
	parts := make([]string, 0, len(s))
	for _, b := range []byte(s) {
		parts = append(parts, fmt.Sprintf("%x", b))
	}
	return strings.Join(parts, " ")
}

const savedFilter = `<?xml version='1.0' encoding='UTF-8'?>
<mdElement elementClass='DiagramProperties'>
  <mdElement elementClass='BooleanProperty'>
    <propertyID>SAVE_FILTER_VALUE</propertyID>
    <value xmi:value='true'/>
  </mdElement>
  <mdElement elementClass='BooleanProperty'>
    <propertyID>OPTION_FILTER_WILDCARD</propertyID>
    <value xmi:value='true'/>
  </mdElement>
  <mdElement elementClass='ChoiceProperty'>
    <propertyID>OPTION_FILTER_COLUMN_INDEXES</propertyID>
    <value>%s</value>
    <choice xmi:value='0^1^2^3^4^5^6'/>
    <index xmi:value='-1'/>
  </mdElement>
  <mdElement elementClass='StringProperty'>
    <propertyID>OPTION_FILTER_SEARCHING_TEXT</propertyID>
    <value>%s</value>
  </mdElement>
</mdElement>`

func tableDocument(applications, properties string) []byte {
	return []byte(`<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:MagicDraw_Profile="http://www.omg.org/spec/UML/20131001/MagicDrawProfile"
         xmlns:Req_Profile="http://example.com/schemas/Req_Profile.xmi"
         xmlns:diagram="http://www.example.com/tool/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Profile" xmi:id="_prof" name="Req Profile" URI="http://example.com/schemas/Req_Profile.xmi">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_st_props" name="Properties">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_st_props_key" name="Key"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_st_reviewed" name="Reviewed">
        <generalization xmi:type="uml:Generalization" xmi:id="_st_reviewed_gen" general="_st_props"/>
      </packagedElement>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i1" name="i1" classifier="_a">
        <slot xmi:type="uml:Slot" xmi:id="_i1_s" definingFeature="_a_b">
          <value xmi:type="uml:InstanceValue" xmi:id="_i1_v" instance="_i2"/>
        </slot>
      </packagedElement>
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i2" name="i2" classifier="_a"/>
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i3" name="i3" classifier="_a"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_r" name="R"/>
    </packagedElement>
    <xmi:Extension extender="Example UML Tool 1.0">
      <modelExtension>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_t" name="Timing" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_t_rep" type="SysML Instance Table"` + properties + `>
                <diagramContents xmi:id="_d_t_contents"/>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
      </modelExtension>
    </xmi:Extension>
  </uml:Model>
` + applications + `
</xmi:XMI>`)
}

const timingTable = `
  <MagicDraw_Profile:InstanceTable xmi:id="_tbl" base_Diagram="_d_t" scope="_p" showScopeAsRoot="false" displayMode="Compact tree" excludedElements="_i3 _missing" includeSubtypesOfRowTypes="false">
    <classifiers xmi:idref="_a"/>
    <rowElements xmi:idref="_i1"/>
    <rowElements xmi:idref="_i2"/>
    <expandedRows>0,_i1:1,_i2</expandedRows>
    <expandedRows>x,_i2</expandedRows>
    <expandedRows>NoExpanded</expandedRows>
    <columnIds>_NUMBER_</columnIds>
    <columnIds>QPROP:Element:name</columnIds>
    <columnIds>QPROP:Element:classifier</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Req Profile::Properties&gt;&gt;.Key</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Properties&gt;&gt;.Key</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Other&gt;&gt;.Key</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Req Profile::Reviewed&gt;&gt;.Key</columnIds>
    <columnIds>QPROP:stereotypeTags:&lt;&lt;Reviewed&gt;&gt;.Grade</columnIds>
    <columnIds>QPROP:stereotypeTags:Properties.Key</columnIds>
    <columnIds>QPROP:Element:owner</columnIds>
    <hideColumns>QPROP:Element:owner</hideColumns>
    <columnWidth>35</columnWidth>
    <columnWidth>774</columnWidth>
    <columnWidth>-1</columnWidth>
    <columnWidth>abc</columnWidth>
    <columnWidth>105</columnWidth>
  </MagicDraw_Profile:InstanceTable>`

func TestTableSettingsAreRead(t *testing.T) {
	m, err := Parse(tableDocument(timingTable, ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tables) != 1 {
		t.Fatalf("tables: %d, want 1", len(m.Tables))
	}
	tb := m.Tables[0]
	if tb.Kind != InstanceTable || tb.DisplayMode != DisplayCompactTree || tb.ShowScopeAsRoot || tb.IncludeSubtypes {
		t.Errorf("kind %s mode %q scopeAsRoot %v subtypes %v", tb.Kind, tb.DisplayMode, tb.ShowScopeAsRoot, tb.IncludeSubtypes)
	}
	if got := ids(tb.Rows); got != "_i1 _i2" {
		t.Errorf("rows %q", got)
	}
	if got := ids(tb.Excluded); got != "_i3 _missing?" {
		t.Errorf("excluded %q", got)
	}
	if got := ids(tb.Expanded); got != "_i1 _i2" {
		t.Errorf("expanded %q", got)
	}
	if tb.RowFilter != nil {
		t.Errorf("row filter %+v, want none saved", tb.RowFilter)
	}
	want := []Column{
		{ID: "_NUMBER_", Kind: ColumnTool, Width: 35},
		{ID: "QPROP:Element:name", Kind: ColumnProperty, Property: "name", Width: 774},
		{ID: "QPROP:Element:classifier", Kind: ColumnProperty, Property: "classifier"},
		{ID: "QPROP:stereotypeTags:<<Req Profile::Properties>>.Key", Kind: ColumnStereotypeTag, Profile: "Req Profile", Stereotype: "Properties", Tag: "Key"},
		{ID: "QPROP:stereotypeTags:<<Properties>>.Key", Kind: ColumnStereotypeTag, Stereotype: "Properties", Tag: "Key", Width: 105},
		{ID: "QPROP:stereotypeTags:<<Other>>.Key", Kind: ColumnStereotypeTag, Stereotype: "Other", Tag: "Key"},
		{ID: "QPROP:stereotypeTags:<<Req Profile::Reviewed>>.Key", Kind: ColumnStereotypeTag, Profile: "Req Profile", Stereotype: "Reviewed", Tag: "Key"},
		{ID: "QPROP:stereotypeTags:<<Reviewed>>.Grade", Kind: ColumnStereotypeTag, Stereotype: "Reviewed", Tag: "Grade"},
		{ID: "QPROP:stereotypeTags:Properties.Key", Kind: ColumnUnknown},
		{ID: "QPROP:Element:owner", Kind: ColumnProperty, Property: "owner", Hidden: true},
	}
	// The stereotype and tag each column resolves to: Reviewed inherits Key
	// from Properties, which it specializes, and defines no Grade.
	wantDefs := map[string][2]string{
		"QPROP:stereotypeTags:<<Req Profile::Properties>>.Key": {"_st_props", "_st_props_key"},
		"QPROP:stereotypeTags:<<Properties>>.Key":              {"_st_props", "_st_props_key"},
		"QPROP:stereotypeTags:<<Req Profile::Reviewed>>.Key":   {"_st_reviewed", "_st_props_key"},
		"QPROP:stereotypeTags:<<Reviewed>>.Grade":              {"_st_reviewed", ""},
	}
	if len(tb.Columns) != len(want) {
		t.Fatalf("columns: %d, want %d", len(tb.Columns), len(want))
	}
	for i, c := range tb.Columns {
		w := want[i]
		def, tag := c.Definition, c.TagDefinition
		c.Definition, c.TagDefinition = nil, nil
		if c != w {
			t.Errorf("column %d: %+v, want %+v", i, c, w)
		}
		if got := [2]string{elementID(def), elementID(tag)}; got != wantDefs[c.ID] {
			t.Errorf("column %d: stereotype and tag %q, want %q", i, got, wantDefs[c.ID])
		}
	}
	if len(tb.Malformed) != 0 {
		t.Errorf("malformed %q, want none: an unreadable window or width setting does not refuse the table", tb.Malformed)
	}
	want2 := []string{`expandedRows "x,_i2": not in the form <level>,<id>`, `columnWidth "abc": not a width in pixels or -1`}
	if !slices.Equal(tb.Ignored, want2) {
		t.Errorf("ignored %q, want %q", tb.Ignored, want2)
	}
}

// A column selection the reader cannot count is not read as every column: the
// filter carries the fault for the migration to drop it.
func TestSavedFilterMalformedColumnSelection(t *testing.T) {
	props := ` diagramProperties="` + hexBytes(fmt.Sprintf(savedFilter, "0^bad^2", "Key")) + `"`
	m, err := Parse(tableDocument(timingTable, props))
	if err != nil {
		t.Fatal(err)
	}
	f := m.Tables[0].RowFilter
	if f == nil {
		t.Fatal("no row filter read")
	}
	if f.Columns != nil || f.Malformed != `OPTION_FILTER_COLUMN_INDEXES "0^bad^2": not column indexes joined by ^` {
		t.Errorf("filter columns %v malformed %q", f.Columns, f.Malformed)
	}
	if f.Text != "Key" {
		t.Errorf("filter text %q", f.Text)
	}
}

func TestDiagramPropertiesAreDecoded(t *testing.T) {
	props := ` diagramProperties="` + hexBytes(fmt.Sprintf(savedFilter, "0^1^2", "Key")) + `"`
	m, err := Parse(tableDocument(timingTable, props))
	if err != nil {
		t.Fatal(err)
	}
	d := m.Diagrams[0]
	if p, ok := d.Property("OPTION_FILTER_COLUMN_INDEXES"); !ok || p.Class != "ChoiceProperty" || p.Value != "0^1^2" || strings.Join(p.Choices, ",") != "0^1^2^3^4^5^6" {
		t.Errorf("column indexes property %+v %v", p, ok)
	}
	if p, ok := d.Property("OPTION_FILTER_SEARCHING_TEXT"); !ok || p.Value != "Key" {
		t.Errorf("search text property %+v %v", p, ok)
	}
	if !d.Flag("SAVE_FILTER_VALUE") || !d.Flag("OPTION_FILTER_WILDCARD") || d.Flag("OPTION_FILTER_REGEXP") {
		t.Errorf("flags: save %v wildcard %v regexp %v", d.Flag("SAVE_FILTER_VALUE"), d.Flag("OPTION_FILTER_WILDCARD"), d.Flag("OPTION_FILTER_REGEXP"))
	}
	f := m.Tables[0].RowFilter
	if f == nil {
		t.Fatal("no row filter read")
	}
	if f.Text != "Key" || !f.Wildcard || f.Regexp || f.CaseSensitive || f.FromStart || f.FromEnd || fmt.Sprint(f.Columns) != "[0 1 2]" {
		t.Errorf("row filter %+v", f)
	}
}

func TestDiagramPropertiesSingleChoiceAndGarbage(t *testing.T) {
	props := ` diagramProperties="` + hexBytes(fmt.Sprintf(savedFilter, "6", "Driver")) + `"`
	m, err := Parse(tableDocument(timingTable, props))
	if err != nil {
		t.Fatal(err)
	}
	if f := m.Tables[0].RowFilter; f == nil || f.Text != "Driver" || fmt.Sprint(f.Columns) != "[6]" {
		t.Errorf("row filter %+v", f)
	}
	// An empty value selects no column, so every column is searched; the
	// choices on offer are not a selection.
	m, err = Parse(tableDocument(timingTable, ` diagramProperties="`+hexBytes(fmt.Sprintf(savedFilter, "", "Key"))+`"`))
	if err != nil {
		t.Fatal(err)
	}
	if f := m.Tables[0].RowFilter; f == nil || f.Text != "Key" || f.Columns != nil {
		t.Errorf("row filter over every column %+v", f)
	}
	for _, bad := range []string{"zz 3c", hexBytes("<not xml")} {
		m, err := Parse(tableDocument(timingTable, ` diagramProperties="`+bad+`"`))
		if err != nil {
			t.Fatal(err)
		}
		if d := m.Diagrams[0]; len(d.Properties) != 0 || m.Tables[0].RowFilter != nil {
			t.Errorf("%q: properties %v filter %v, want none", bad, d.Properties, m.Tables[0].RowFilter)
		}
	}
	unsaved := strings.Replace(fmt.Sprintf(savedFilter, "6", "Driver"), "<propertyID>SAVE_FILTER_VALUE</propertyID>\n    <value xmi:value='true'/>", "<propertyID>SAVE_FILTER_VALUE</propertyID>\n    <value xmi:value='false'/>", 1)
	m, err = Parse(tableDocument(timingTable, ` diagramProperties="`+hexBytes(unsaved)+`"`))
	if err != nil {
		t.Fatal(err)
	}
	if f := m.Tables[0].RowFilter; f != nil {
		t.Errorf("unsaved filter read as %+v", f)
	}
}

func elementID(e *Element) string {
	if e == nil {
		return ""
	}
	return e.ID
}
