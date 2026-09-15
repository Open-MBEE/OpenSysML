package xmi

import (
	"strings"
	"testing"
)

const doc = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML">
  <uml:Model xmi:id="m" name="Model">
    <packagedElement xmi:type="uml:Activity" xmi:id="a" name="A" node="n1 n2">
      <node xmi:type="uml:ForkNode" xmi:id="n1" name="fork"/>
      <node xmi:type="uml:JoinNode" xmi:id="n2">
        <incoming xmi:idref="e1"/>
        <incoming xmi:idref="e2"/>
      </node>
      <edge xmi:type="uml:ControlFlow" xmi:id="e1" source="n1" target="n2">
        <guard xmi:type="uml:LiteralBoolean" xmi:id="g" value="true"/>
      </edge>
      <edge xmi:type="uml:ControlFlow" xmi:id="e2" source="n1" target="n2"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="c">
        <body>text body</body>
      </ownedComment>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="k" name="K" classifierBehavior="a"/>
    <behavior xmi:type="uml:Activity" href="lib.xmi#WriteLine"/>
  </uml:Model>
</xmi:XMI>
`

func parse(t *testing.T) *Document {
	t.Helper()
	d, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestParseIndexesAndNests(t *testing.T) {
	d := parse(t)
	if d.Root.Tag != "XMI" || d.Root.Attr("version") != "20131001" {
		t.Errorf("root = %s %q", d.Root.Tag, d.Root.Attrs)
	}
	a := d.ByID("a")
	if a == nil || a.Type != "uml:Activity" || a.Name() != "A" || a.Tag != "packagedElement" {
		t.Fatalf("a = %s", a.Describe())
	}
	if a.Parent != d.ByID("m") || len(a.Children) != 5 || a.Line != 4 {
		t.Errorf("a: parent %s, %d children, line %d", a.Parent.Describe(), len(a.Children), a.Line)
	}
	if d.ByID("") != nil || d.ByID("nope") != nil || (*Document)(nil).ByID("a") != nil {
		t.Error("ByID resolved something it should not")
	}
	if got := d.ByID("c").First("body").Text; strings.TrimSpace(got) != "text body" {
		t.Errorf("text = %q", got)
	}
}

func TestAccessors(t *testing.T) {
	d := parse(t)
	a := d.ByID("a")
	if got := a.Refs("node"); len(got) != 2 || got[0] != "n1" || got[1] != "n2" {
		t.Errorf("Refs(node) = %q", got)
	}
	if got := d.ByID("n2").Refs("incoming"); len(got) != 2 || got[0] != "e1" || got[1] != "e2" {
		t.Errorf("Refs(incoming) = %q", got)
	}
	e1 := d.ByID("e1")
	if e1.Ref("source") != "n1" || e1.Ref("target") != "n2" || e1.Ref("guard") != "" || e1.Ref("nothing") != "" {
		t.Errorf("Ref: source %q target %q guard %q", e1.Ref("source"), e1.Ref("target"), e1.Ref("guard"))
	}
	if e1.First("guard").Attr("value") != "true" || e1.First("missing") != nil {
		t.Error("First")
	}
	if len(a.Tagged("node")) != 2 || len(a.Tagged("edge")) != 2 || len(a.Tagged("none")) != 0 {
		t.Error("Tagged")
	}
	var href *Element
	d.Root.Walk(func(e *Element) bool {
		if e.Href() != "" {
			href = e
		}
		return true
	})
	if href == nil || href.Href() != "lib.xmi#WriteLine" || href.ID != "" {
		t.Errorf("href = %s", href.Describe())
	}
	if n := len(a.Descendants()); n != 9 {
		t.Errorf("descendants = %d, want 9", n)
	}
	var seen int
	d.Root.Walk(func(e *Element) bool {
		seen++
		return e.ID != "a"
	})
	if seen != 3 {
		t.Errorf("Walk stopped after %d, want 3", seen)
	}
	var nilElem *Element
	if nilElem.Attr("x") != "" || nilElem.Describe() != "<nil>" {
		t.Error("nil element accessors")
	}
	if got := d.ByID("n1").Describe(); got != `uml:ForkNode "fork" (n1)` {
		t.Errorf("Describe = %q", got)
	}
	if got := d.ByID("c").First("body").Describe(); got != "body" {
		t.Errorf("Describe(untyped) = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"empty":       "",
		"unbalanced":  `<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><a>`,
		"two roots":   `<a/><b/>`,
		"dup id":      `<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><a xmi:id="x"/><b xmi:id="x"/></xmi:XMI>`,
		"not xml":     `hello`,
		"bad closing": `<a></b>`,
	}
	for name, src := range cases {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestParseAcceptsEveryXMINamespaceVersion(t *testing.T) {
	for _, version := range []string{"20110701", "20131001"} {
		src := `<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/` + version + `" xmlns:uml="http://www.omg.org/spec/UML/20110701">
  <uml:Model xmi:type="uml:Model" xmi:id="_0" name="Lib">
    <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="F-plus" name="+"/>
  </uml:Model>
</xmi:XMI>`
		d, err := Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("%s: %v", version, err)
		}
		e := d.ByID("F-plus")
		if e == nil || e.Type != "uml:FunctionBehavior" || e.Name() != "+" {
			t.Fatalf("%s: ByID(F-plus) = %s, want the function behavior", version, e.Describe())
		}
	}
	other := `<x:XMI xmlns:x="http://example.com/not-xmi"><e x:id="a" x:type="t"/></x:XMI>`
	d, err := Parse(strings.NewReader(other))
	if err != nil {
		t.Fatal(err)
	}
	if d.ByID("a") != nil {
		t.Fatal("an id in a foreign namespace must not be indexed")
	}
}
