package Fleet {
	private import ScalarValues::*;
	abstract part def Vehicle {
		attribute mass : Real;
	}
	part def Wheel;
	part def Truck :> Vehicle {
		part wheels : Wheel [6..*];
		attribute axles : Integer [2];
	}
	part def Bike :> Vehicle {
		part wheels : Wheel [2];
	}
}
