package transformation

import (
	"os"
	"path/filepath"
	"testing"
)

// syntheticModel is a minimal Mappings package exercising the cases a full
// extraction would not isolate: an inherited `from`/`to`, a mapping class
// nested inside another mapping class, and a `to` resolved through a general.
const syntheticModel = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20161101" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Package xmi:id="SysMLv1Tov2" name="SysMLv1Tov2">
    <packagedElement xmi:id="Mappings" xmi:type="uml:Package" name="Mappings">
      <packagedElement xmi:id="Mappings-Foundations" xmi:type="uml:Package" name="Foundations">
        <packagedElement xmi:id="Mappings-Foundations-Mapping" xmi:type="uml:Class" isAbstract="true" name="Mapping">
          <ownedAttribute xmi:id="Mappings-Foundations-Mapping-from" xmi:type="uml:Property" name="from">
            <type href="https://www.omg.org/spec/UML/20161101/UML.xmi#Element" />
          </ownedAttribute>
          <ownedAttribute xmi:id="Mappings-Foundations-Mapping-to" xmi:type="uml:Property" name="to">
            <type href="https://www.omg.org/spec/SysML/20250201/SysML.xmi#Root-Elements-Element" />
          </ownedAttribute>
        </packagedElement>
      </packagedElement>
      <packagedElement xmi:id="Mappings-Initializers" xmi:type="uml:Package" name="Initializers">
        <packagedElement xmi:id="Mappings-Initializers-ToUsage_Init" xmi:type="uml:Class" name="ToUsage_Init">
          <ownedAttribute xmi:id="Mappings-Initializers-ToUsage_Init-to" xmi:type="uml:Property" name="to">
            <type href="https://www.omg.org/spec/SysML/20250201/SysML.xmi#Core-Usages-Usage" />
          </ownedAttribute>
        </packagedElement>
      </packagedElement>
      <packagedElement xmi:id="Mappings-PkgA" xmi:type="uml:Package" name="PkgA">
        <packagedElement xmi:id="Mappings-PkgA-Outer_Mapping" xmi:type="uml:Class" isAbstract="true" name="Outer_Mapping">
          <generalization xmi:id="g1" xmi:type="uml:Generalization">
            <general xmi:idref="Mappings-Foundations-Mapping" />
          </generalization>
          <nestedClassifier xmi:id="Mappings-PkgA-Outer_Mapping-Inner_Mapping" xmi:type="uml:Class" name="Inner_Mapping">
            <generalization xmi:id="g2" xmi:type="uml:Generalization">
              <general xmi:idref="Mappings-PkgA-Outer_Mapping" />
            </generalization>
            <ownedAttribute xmi:id="Mappings-PkgA-Inner_Mapping-from" xmi:type="uml:Property" name="from">
              <type xmi:idref="Mappings-Initializers-ToUsage_Init" />
            </ownedAttribute>
            <ownedOperation xmi:id="op1" xmi:type="uml:Operation" name="result">
              <bodyCondition xmi:id="bc1" xmi:type="uml:Constraint" name="bodyCondition">
                <specification xmi:id="s1" xmi:type="uml:OpaqueExpression" body="result = from" language="OCL2.0" />
                <specification xmi:id="s2" xmi:type="uml:OpaqueExpression" body="ignored" language="English" />
              </bodyCondition>
            </ownedOperation>
          </nestedClassifier>
        </packagedElement>
        <packagedElement xmi:id="Mappings-PkgA-UsesInit_Mapping" xmi:type="uml:Class" name="UsesInit_Mapping">
          <generalization xmi:id="g3" xmi:type="uml:Generalization">
            <general xmi:idref="Mappings-Initializers-ToUsage_Init" />
          </generalization>
          <generalization xmi:id="g4" xmi:type="uml:Generalization">
            <general xmi:idref="Mappings-Foundations-Mapping" />
          </generalization>
        </packagedElement>
      </packagedElement>
    </packagedElement>
  </uml:Package>
</xmi:XMI>
`

func writeSynthetic(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SysMLv1Tov2.xmi")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExtractsMappingClasses checks a synthetic model end to end: inherited
// and own `from`/`to`, the nested classifier's qualified name and package, the
// document-order generals, and the OCL digest over OCL2.0 bodies only.
func TestExtractsMappingClasses(t *testing.T) {
	out, err := extract(writeSynthetic(t, syntheticModel))
	if err != nil {
		t.Fatal(err)
	}
	if out.Packages != 4 || out.Classes != 5 || out.OCLBodies != 1 {
		t.Fatalf("counts = %d packages, %d classes, %d bodies", out.Packages, out.Classes, out.OCLBodies)
	}
	if len(out.Mappings) != 3 {
		t.Fatalf("extracted %d mappings, want 3", len(out.Mappings))
	}
	byName := map[string]Mapping{}
	for _, m := range out.Mappings {
		byName[m.Name] = m
	}
	inner := byName["Inner_Mapping"]
	if inner.Package != "PkgA" || inner.Qualified != "PkgA::Outer_Mapping::Inner_Mapping" {
		t.Errorf("inner: package %q qualified %q", inner.Package, inner.Qualified)
	}
	if inner.From != "ToUsage_Init" {
		t.Errorf("inner.from = %q, want the in-file type's name", inner.From)
	}
	if inner.To != "Element" {
		t.Errorf("inner.to = %q, want the inherited href fragment's last segment", inner.To)
	}
	if len(inner.Generals) != 1 || inner.Generals[0] != "Outer_Mapping" {
		t.Errorf("inner.generals = %v", inner.Generals)
	}
	if len(inner.Operations) != 1 || inner.Operations[0] != "result" {
		t.Errorf("inner.operations = %v", inner.Operations)
	}
	outer := byName["Outer_Mapping"]
	if !outer.Abstract || outer.From != "Element" || outer.To != "Element" {
		t.Errorf("outer: abstract %v from %q to %q", outer.Abstract, outer.From, outer.To)
	}
	init := byName["UsesInit_Mapping"]
	if init.From != "Element" {
		t.Errorf("init.from = %q, want the breadth-first inherited value", init.From)
	}
	if init.To != "Usage" {
		t.Errorf("init.to = %q, want Usage inherited through ToUsage_Init", init.To)
	}
	if len(init.Generals) != 2 || init.Generals[0] != "ToUsage_Init" || init.Generals[1] != "Mapping" {
		t.Errorf("init.generals = %v, want document order", init.Generals)
	}
}

// TestExtractRejectsDuplicateNames: the census keys rows by name, so a model
// repeating one is an error, not a silent merge.
func TestExtractRejectsDuplicateNames(t *testing.T) {
	dup := `<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20161101" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Package xmi:id="SysMLv1Tov2">
    <packagedElement xmi:id="Mappings" xmi:type="uml:Package" name="Mappings">
      <packagedElement xmi:id="p1" xmi:type="uml:Package" name="A">
        <packagedElement xmi:id="c1" xmi:type="uml:Class" name="Same_Mapping" />
      </packagedElement>
      <packagedElement xmi:id="p2" xmi:type="uml:Package" name="B">
        <packagedElement xmi:id="c2" xmi:type="uml:Class" name="Same_Mapping" />
      </packagedElement>
    </packagedElement>
  </uml:Package>
</xmi:XMI>
`
	if _, err := extract(writeSynthetic(t, dup)); err == nil {
		t.Fatal("two _Mapping classes of one name must fail")
	}
}
