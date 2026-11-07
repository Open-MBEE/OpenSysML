package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestReflectiveAttributesBelongToTheirMetaclasses(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Part;
		port def Port;
		state def Parallel parallel { state a; state b; }
		state def Plain;
		part p {
			state parallelState parallel { state a; state b; }
			state plainState { state inner; }
		}
		constant part frozen;
	}`)
	for _, tc := range []struct {
		path    string
		feature string
		present bool
		value   bool
	}{
		{path: "P::Part", feature: "isConstant"},
		{path: "P::Port", feature: "isConstant"},
		{path: "P::Part", feature: "isParallel"},
		{path: "P::Port", feature: "isParallel"},
		{path: "P::Parallel", feature: "isParallel", present: true, value: true},
		{path: "P::Plain", feature: "isParallel", present: true},
		{path: "P::p::plainState::inner", feature: "isParallel", present: true},
		{path: "P::p::plainState::inner", feature: "isSufficient", present: true},
		{path: "P::p::parallelState", feature: "isParallel", present: true, value: true},
		{path: "P::p::plainState", feature: "isParallel", present: true},
		{path: "P::p", feature: "isParallel"},
		{path: "P::p", feature: "isConstant", present: true},
		{path: "P::frozen", feature: "isConstant", present: true, value: true},
	} {
		t.Run(tc.path+"."+tc.feature, func(t *testing.T) {
			sym := nestedSym(t, root, tc.path)
			got, ok := m.ReflectiveFeatureValue(sym, tc.feature)
			if ok != tc.present {
				t.Fatalf("%s.%s = %+v (ok %t), want ok %t", tc.path, tc.feature, got, ok, tc.present)
			}
			if ok && (got.Kind != symbols.FilterValueBool || got.Bool != tc.value) {
				t.Errorf("%s.%s = %+v, want %t", tc.path, tc.feature, got, tc.value)
			}
		})
	}
}
