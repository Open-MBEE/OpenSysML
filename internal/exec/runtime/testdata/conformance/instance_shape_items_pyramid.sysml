// `ShapeItems::Pyramid::apex :> vertices = wall.apex` declares no multiplicity, so it is bound by
// `vertices [0..*]`, not a default of one (SysML v2 §7.6.3): a wall's two apex vertices are no violation.
package test {
	part def P {
		item :>> shape : ShapeItems::Pyramid {
			:>> height = 1 [SI::m];
		}
	}
}
