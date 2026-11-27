package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A KerML type that is no feature reflects Type::isAbstract (KerML 8.3.3.1.10) as declared.
func TestKerMLTypeReflectsIsAbstract(t *testing.T) {
	const src = `package P {
    abstract struct A;
    struct B;
}`
	for _, fixture := range reflectiveFixtures(t, "kerml-abstract.kerml", source.KindKerML, src) {
		for name, want := range map[string]bool{"P::A": true, "P::B": false} {
			value, ok := fixture.model.ReflectiveFeatureValue(nestedSym(t, fixture.root, name), "isAbstract")
			if !ok || value.Kind != symbols.FilterValueBool || value.Bool != want {
				t.Errorf("%s isAbstract = %+v (known %v), want %v", name, value, ok, want)
			}
		}
	}
}
