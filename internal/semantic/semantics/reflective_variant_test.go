package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A variant is an owned member but not an owned feature of its variation, so it
// shares the featuringTypes of a variation usage and takes none from a
// variation definition (SysML v2 checkUsageVariationUsageTypeFeaturing).
func TestReflectiveVariantFeaturingTypes(t *testing.T) {
	const src = `package P {
		part def E;
		part def Car {
			variation part engine : E {
				variant part e1 : E;
				variant part e2 : E;
			}
			part wheel : E;
		}
		variation part def EngineChoices :> E {
			variant part petrol : E;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "variant.sysml", source.KindSysML, src) {
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Car::engine"), "featuringType", "P::Car")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Car::engine::e1"), "featuringType", "P::Car")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Car::wheel"), "featuringType", "P::Car")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::EngineChoices::petrol"), "featuringType")
	}
}
