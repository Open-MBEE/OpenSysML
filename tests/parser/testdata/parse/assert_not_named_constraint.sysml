// `assert not <name> { … }` negates the named constraint: `not` is the
// operator of the asserted expression, not a declaration modifier.
package AssertNotNamedConstraint {
	constraint def MassLimit {
		in mass;
		in massLimit;
		mass <= massLimit
	}

	part def Vehicle {
		attribute mass;
	}

	part def Context {
		part vehicle3 : Vehicle;
		part vehicle4 : Vehicle;

		constraint massLimitation : MassLimit;

		assert not massLimitation {
			in :>> mass = vehicle3.mass;
			in :>> massLimit = vehicle4.mass;
		}
	}
}
