package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A feature its clause leaves nameless answers to the name of the feature it
// implicitly redefines (KerML 1.1 §8.2.4): a positional parameter to its
// behavior's, a connector end to the end it occupies in its definition, or to
// the binary library's source/target. Element::name and qualifiedName state it;
// declaredName stays empty.
func TestReflectiveNameOfAnImplicitlyRedefiningFeature(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package Names {
	item def Scene; item def Picture;
	action def Shoot { in image : Scene; out picture : Picture; }
	action shoot : Shoot { in item; out item picture; }
	part def LogicalElement; part def PhysicalElement;
	part torqueGenerator : LogicalElement;
	part powerTrain : PhysicalElement;
	allocation def LogicalToPhysical { end logical : LogicalElement; end physical : PhysicalElement; }
	allocation torqueGenAlloc : LogicalToPhysical allocate torqueGenerator to powerTrain;
	part a { port o; } part b { port i; }
	connection c connect a.o to b.i;
}`)
	pkg, _ := root.LookupLocal("Names")
	shoot := member(t, m, pkg, "shoot")
	features, _ := m.ReflectiveElements(shoot, "ownedFeature")
	if len(features) == 0 {
		t.Fatal("shoot owns no feature")
	}
	image := features[0]
	for feature, want := range map[string]string{
		"name": "image", "declaredName": "", "qualifiedName": "Names::shoot::image",
	} {
		assertReflectiveString(t, m, image, feature, want)
	}
	for connector, names := range map[string][]string{"torqueGenAlloc": {"logical", "physical"}, "c": {"source", "target"}} {
		sym := member(t, m, pkg, connector)
		ends, _ := m.ReflectiveElements(sym, "connectorEnd")
		if len(ends) != 2 {
			t.Fatalf("%s has %d ends, want 2", connector, len(ends))
		}
		for i, want := range names {
			end := ends[i]
			assertReflectiveString(t, m, end, "name", want)
			assertReflectiveString(t, m, end, "declaredName", "")
			assertReflectiveString(t, m, end, "qualifiedName", "Names::"+connector+"::"+want)
		}
	}
}

// An extensionless buffer reads as KerML when only KerML's keywords declare it, as a
// graph-alone rendering of a KerML model is read back; mixed text stays SysML.
func TestExtensionlessBufferReadsAsKerMLOnlyWhenPurelyKerML(t *testing.T) {
	for _, tc := range []struct {
		name, pkg, src string
		kerml          bool
	}{
		{"pure kerml", "Calls", "package Calls {\n\tfunction pick { in a : Integer; return : Integer; }\n}\n", true},
		{"kerml features", "Calls", "package Calls {\n\tfunction pick { feature x; step s; connector c; }\n\tstruct S { feature f; }\n}\n", true},
		{"mixed", "Demo", "package Demo {\n\tpart p;\n\tfunction pick { return : Integer; }\n}\n", false},
		{"pure sysml", "Demo", "package Demo {\n\tpart p;\n\tcalc def pick { return : Integer; }\n}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, root := buildModelWithStdlibNamedKind(t, "buffer", source.KindUnknown, tc.src)
			pkg, ok := root.LookupLocal(tc.pkg)
			if !ok || pkg == nil {
				t.Fatalf("no package %s", tc.pkg)
			}
			if got := m.isKerMLDoc(pkg); got != tc.kerml {
				t.Errorf("isKerMLDoc = %v, want %v", got, tc.kerml)
			}
		})
	}
}
