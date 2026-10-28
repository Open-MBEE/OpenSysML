package P {
	private import ScalarValues::*;
	attribute xs : Integer[0..*] = 4;
	part def D { attribute xs : Integer[0..*] = 4; }
	part d : D;
}
