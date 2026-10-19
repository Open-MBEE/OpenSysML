// `ShapeItems::Pyramid::apex :> vertices = wall.apex` declares no multiplicity and subsets a
// type-owned feature, so it is bound by the `[0..*]` of `vertices`, not by a default of one
// (SysML v2 §7.6.3, KerML 1.0 §8.4.4.12.1): the two apex vertices of a wall are no violation.
package test {
	part def P {
		item :>> shape : ShapeItems::Pyramid {
			:>> height = 1 [SI::m];
		}
	}
}
