package migrate

import (
	"strconv"
	"strings"
	"testing"
)

// treeModel wraps an Expression tree as the constraint of a block whose Real
// attributes a and b, and Mode attribute m, the tree may name.
func treeModel(spec string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_on" name="On"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_blk" name="Blk">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="a">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_mm" name="m" type="_mode"/>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_rule" name="r" constrainedElement="_blk">
        ` + spec + `
      </ownedRule>
    </packagedElement>
  </uml:Model>
  <sysml:Block xmi:id="_stereo" base_Class="_blk"/>
</xmi:XMI>`
}

var leafCount int

// leaf is an Expression node with a bare symbol, under a fresh id.
func leaf(symbol string) string {
	leafCount++
	return `<operand xmi:type="uml:Expression" xmi:id="_leaf` + strconv.Itoa(leafCount) + `" symbol="` + symbol + `"/>`
}

func TestExpressionTreeLowering(t *testing.T) {
	for _, tc := range []struct{ name, spec, want string }{
		{"arithmetic and comparison",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="&lt;=">
			   <operand xmi:type="uml:Expression" xmi:id="_s1" symbol="+">` + leaf("a") + `<operand xmi:type="uml:LiteralInteger" xmi:id="_l" value="2"/></operand>
			   <operand xmi:type="uml:Expression" xmi:id="_s2" symbol="*">` + leaf("b") + `<operand xmi:type="uml:LiteralReal" xmi:id="_r" value="1.5"/></operand>
			 </specification>`,
			"constraint r { a + 2 <= b * 1.5 }"},
		{"unary minus and not",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="not">
			   <operand xmi:type="uml:Expression" xmi:id="_s1" symbol="&lt;">` + leaf("a") + `<operand xmi:type="uml:Expression" xmi:id="_s2" symbol="-">` + leaf("b") + `</operand></operand>
			 </specification>`,
			"constraint r { not (a < -b) }"},
		{"nested boolean operators fold several operands",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="and">
			   <operand xmi:type="uml:Expression" xmi:id="_s1" symbol="&gt;">` + leaf("a") + `<operand xmi:type="uml:LiteralInteger" xmi:id="_l" value="0"/></operand>
			   <operand xmi:type="uml:Expression" xmi:id="_s2" symbol="&gt;">` + leaf("b") + `<operand xmi:type="uml:LiteralInteger" xmi:id="_l2" value="0"/></operand>
			   <operand xmi:type="uml:Expression" xmi:id="_s3" symbol="!=">` + leaf("a") + leaf("b") + `</operand>
			 </specification>`,
			"constraint r { a > 0 and b > 0 and a != b }"},
		{"instance value of an enumeration literal",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="==">` + leaf("m") + `<operand xmi:type="uml:InstanceValue" xmi:id="_iv" instance="_on"/></specification>`,
			"constraint r { m == Mode::On }"},
		{"function of the translated table",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="&gt;=">
			   <operand xmi:type="uml:Expression" xmi:id="_s1" symbol="Math.abs">` + leaf("a") + `</operand>
			   <operand xmi:type="uml:Expression" xmi:id="_s2" symbol="min">` + leaf("a") + leaf("b") + `</operand>
			 </specification>`,
			"constraint r { RealFunctions::abs(a) >= RealFunctions::min(a, b) }"},
		{"script operand inside a tree",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="&gt;">` + leaf("a") +
				`<operand xmi:type="uml:OpaqueExpression" xmi:id="_o"><body>b / 2</body><language>JavaScript</language></operand></specification>`,
			"constraint r { a > b / 2 }"},
		{"unknown operator symbol is refused",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="implies">` + leaf("a") + leaf("b") + `</specification>`,
			`not migrated: Constraint 'r' implies(a, b) — the UML Expression tree has no v2 form: the call "implies" is not in the translated function table`},
		{"interval operand is refused",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="&lt;">` + leaf("a") + `<operand xmi:type="uml:Interval" xmi:id="_i"/></specification>`,
			`the UML Expression tree has no v2 form: the construct "<Interval>" is outside the translated subset: a UML Interval has no v2 expression`},
		{"reserved word in an opaque operand is refused",
			`<specification xmi:type="uml:Expression" xmi:id="_s" symbol="==">` + leaf("a") +
				`<operand xmi:type="uml:OpaqueExpression" xmi:id="_o"><body>null</body><language>JavaScript</language></operand></specification>`,
			`the UML Expression tree has no v2 form: the construct "null"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("tree.xmi", []byte(treeModel(tc.spec)))
			if err != nil {
				t.Fatal(err)
			}
			if got := string(r.Notation); !strings.Contains(got, tc.want) {
				t.Errorf("notation lacks %q:\n%s", tc.want, got)
			}
		})
	}
}

func TestTreePlaceholderNames(t *testing.T) {
	l := &treeLowering{leaves: map[string]opaqueRef{}}
	for name, want := range map[string]string{"On": "On", "new": "_new", "true": "_true", "3rd": "_3rd", "a-b": "a_b", "": "_"} {
		if got := l.placeholder(name); got != want {
			t.Errorf("placeholder(%q) = %q, want %q", name, got, want)
		}
		l.leaves[want] = opaqueRef{}
	}
	if got := l.placeholder("On"); got != "On2" {
		t.Errorf("second placeholder for On = %q, want On2", got)
	}
}
