package xmi

import (
	"reflect"
	"strings"
	"testing"
)

// elementForm writes scalar properties and references as child elements, the
// way XMI 2.5.1 § 9.4.4 allows and the OMG UML XMI files do.
const elementForm = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Model xmi:type="uml:Model" xmi:id="m">
    <name>Model</name>
    <packagedElement xmi:type="uml:Class" xmi:id="k">
      <name>Vehicle</name>
      <isAbstract>true</isAbstract>
      <ownedAttribute xmi:type="uml:Property" xmi:id="p">
        <name>engine</name>
        <aggregation>composite</aggregation>
        <type xmi:idref="e"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="lo" value="1"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="hi">
          <value>2</value>
        </upperValue>
        <defaultValue xmi:type="uml:LiteralString" xmi:id="dv">
          <value>none</value>
        </defaultValue>
      </ownedAttribute>
      <ownedComment xmi:type="uml:Comment" xmi:id="c">
        <body>text body</body>
        <annotatedElement xmi:idref="k"/>
        <annotatedElement xmi:idref="p"/>
      </ownedComment>
      <ownedRule xmi:type="uml:Constraint" xmi:id="r">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="spec">
          <body>mass &gt; 0</body>
          <body>mass &lt; 10</body>
          <language>OCL</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="e" name="Engine">
      <name>Motor</name>
      <generalization xmi:type="uml:Generalization" xmi:id="g">
        <general href="lib.xmi#Machine"/>
      </generalization>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="a">
      <memberEnd xmi:idref="p"/>
      <memberEnd xmi:idref="q"/>
    </packagedElement>
  </uml:Model>
</xmi:XMI>
`

func parseElementForm(t *testing.T) *Document {
	t.Helper()
	d, err := Parse(strings.NewReader(elementForm))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAttrReadsElementSerialization(t *testing.T) {
	d := parseElementForm(t)
	k := d.ByID("k")
	if got := k.Name(); got != "Vehicle" {
		t.Errorf("Name() = %q", got)
	}
	if got := k.Attr("isAbstract"); got != "true" {
		t.Errorf("Attr(isAbstract) = %q", got)
	}
	p := d.ByID("p")
	if got := p.Attr("aggregation"); got != "composite" {
		t.Errorf("Attr(aggregation) = %q", got)
	}
	if got := d.ByID("c").Attr("body"); got != "text body" {
		t.Errorf("Attr(body) = %q", got)
	}
	// The attribute wins when both forms state a property.
	if got := d.ByID("e").Name(); got != "Engine" {
		t.Errorf("Name() with both forms = %q", got)
	}
	if got := d.ByID("e").Values("name"); !reflect.DeepEqual(got, []string{"Engine", "Motor"}) {
		t.Errorf("Values(name) = %q", got)
	}
	// Repeated children are the values of a multi-valued property.
	spec := d.ByID("spec")
	if got := spec.Values("body"); !reflect.DeepEqual(got, []string{"mass > 0", "mass < 10"}) {
		t.Errorf("Values(body) = %q", got)
	}
	if got := spec.Attr("body"); got != "mass > 0" {
		t.Errorf("Attr(body) = %q", got)
	}
	if got := spec.Values("missing"); got != nil {
		t.Errorf("Values(missing) = %q", got)
	}
	if got := (*Element)(nil).Values("name"); got != nil {
		t.Errorf("nil Values = %q", got)
	}
}

func TestRefReadsElementSerialization(t *testing.T) {
	d := parseElementForm(t)
	if got := d.ByID("p").Ref("type"); got != "e" {
		t.Errorf("Ref(type) = %q", got)
	}
	if got := d.ByID("a").Refs("memberEnd"); !reflect.DeepEqual(got, []string{"p", "q"}) {
		t.Errorf("Refs(memberEnd) = %q", got)
	}
	if got := d.ByID("c").Refs("annotatedElement"); !reflect.DeepEqual(got, []string{"k", "p"}) {
		t.Errorf("Refs(annotatedElement) = %q", got)
	}
	// A reference child carries no text, so it is not a scalar value.
	if got := d.ByID("p").Attr("type"); got != "" {
		t.Errorf("Attr(type) = %q", got)
	}
	// An href child is a reference to another document, not a scalar either.
	general := d.ByID("g").First("general")
	if general.IsScalar() || general.Href() != "lib.xmi#Machine" {
		t.Errorf("general = %s scalar=%v", general.Describe(), general.IsScalar())
	}
}

func TestOwnedChildrenAreNotScalars(t *testing.T) {
	d := parseElementForm(t)
	p := d.ByID("p")
	// A child with xmi:type or xmi:id is an owned element, whatever else it carries.
	for _, tag := range []string{"lowerValue", "upperValue", "defaultValue"} {
		c := p.First(tag)
		if c == nil || c.IsScalar() {
			t.Errorf("%s: owned element read as scalar", tag)
		}
		if got := p.Attr(tag); got != "" {
			t.Errorf("Attr(%s) = %q", tag, got)
		}
	}
	// The value an owned element serializes as a child belongs to that element.
	if got := d.ByID("hi").Attr("value"); got != "2" {
		t.Errorf("upperValue value = %q", got)
	}
	if got := d.ByID("dv").Attr("value"); got != "none" {
		t.Errorf("defaultValue value = %q", got)
	}
	if got := d.ByID("lo").Attr("value"); got != "1" {
		t.Errorf("lowerValue value = %q", got)
	}
	if got := d.ByID("k").Attr("ownedAttribute"); got != "" {
		t.Errorf("Attr(ownedAttribute) = %q", got)
	}
	if got := d.ByID("r").Attr("specification"); got != "" {
		t.Errorf("Attr(specification) = %q", got)
	}
	// Scalar children stay in the tree for callers that walk it.
	if name := d.ByID("k").First("name"); name == nil || !name.IsScalar() || name.Text != "Vehicle" {
		t.Errorf("name child = %+v", name)
	}
	// A root and a nil element are not scalars.
	if d.Root.IsScalar() || (*Element)(nil).IsScalar() {
		t.Error("root or nil read as scalar")
	}
	// A childless, attribute-less owned element with xmi:type is still owned.
	empty, err := Parse(strings.NewReader(`<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><a xmi:id="a"><b xmi:type="uml:Class"/><c/></a></xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	a := empty.ByID("a")
	if a.First("b").IsScalar() || !a.First("c").IsScalar() || a.Attr("c") != "" || a.Attr("b") != "" {
		t.Errorf("b scalar=%v c scalar=%v", a.First("b").IsScalar(), a.First("c").IsScalar())
	}
	if got := a.Values("c"); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("Values(c) = %q", got)
	}
}

func TestXMLMetadataDoesNotMakeAChildOwned(t *testing.T) {
	d, err := Parse(strings.NewReader(`<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001">
  <specification xmi:type="uml:OpaqueExpression" xmi:id="s">
    <body xml:space="preserve">x + 1</body>
    <body xml:lang="en">two</body>
    <lowerValue value="1"/>
  </specification>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	s := d.ByID("s")
	// xml:space and xml:lang describe the text; the child still serializes the property.
	body := s.First("body")
	if !body.IsScalar() || body.XMLAttrs["space"] != "preserve" || len(body.Attrs) != 0 {
		t.Errorf("decorated body = %+v", body)
	}
	if got := s.Attr("body"); got != "x + 1" {
		t.Errorf("Attr(body) = %q", got)
	}
	if got := s.Values("body"); !reflect.DeepEqual(got, []string{"x + 1", "two"}) {
		t.Errorf("Values(body) = %q", got)
	}
	// Any other attribute makes the child an element of its own.
	if lo := s.First("lowerValue"); lo.IsScalar() || s.Attr("lowerValue") != "" {
		t.Errorf("lowerValue read as scalar: %+v", lo)
	}
}
