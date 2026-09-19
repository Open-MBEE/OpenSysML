// A part whose nested part redeclares a name the outer part also declares, so
// inspecting a member of the nested part is only right if it is answered
// against the nested object.
package Nested {
	part def Engine {
		attribute power = 300.0;
	}

	part def Car {
		attribute mass = 1500.0;
		part engine : Engine {
			attribute mass = 5.0;
			constraint light {
				mass < 10.0
			}
		}
	}
}
