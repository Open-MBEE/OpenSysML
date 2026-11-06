package RefParts {
	part def Engine;
	part def Person;

	part def Car {
		part engine : Engine;
		ref part driver : Person;
		attribute mass : ScalarValues::Real;
		port p;
		action drive { in speed : ScalarValues::Real; }
	}
}
