package AnonymousAssertions {
	attribute def MassValue;
	constraint def MassConstraint {
		in attribute total : MassValue;
		in attribute limit : MassValue;
		total <= limit
	}
	constraint massConstraint : MassConstraint {
		in attribute total : MassValue;
		in attribute limit : MassValue;
		total <= limit
	}
	part def Vehicle {
		attribute mass : MassValue;
		attribute maxMass : MassValue;
		assert massConstraint {
			in total = mass;
			in limit = maxMass;
		}
		assert constraint {
			mass <= maxMass
		}
		assert not constraint tooHeavy : MassConstraint {
			in total = mass;
			in limit = maxMass;
		}
		assert not massConstraint;
		assert massConstraint;
		assert constraint c : MassConstraint;
	}
}
