// A constructor expression `new T(...)` (KerML §7.4.9) materializes a fresh object
// each time it is evaluated: two objects of one definition coexist in one context,
// distinct from each other and from any declared part, each a live occurrence that
// starts its classifier's behaviors, held by the feature the action writes it into
// and in the extent of its definition.
package test {
	private import ScalarValues::*;
	private import OccurrenceFunctions::*;
	private import SequenceFunctions::*;

	part def Car {
		attribute n : Integer;
		attribute wheels : Integer = 4;
		exhibit state running { entry; then idle; state idle; }
	}

	part def Fleet {
		part cars : Car[0..*];
		part flagship : Car;
		attribute count : Integer;
		attribute extent : Integer;
		attribute distinct : Boolean;
		attribute secondN : Integer;
		attribute secondWheels : Integer;
		attribute secondAlive : Boolean;
		attribute secondIsCar : Boolean;

		perform action build {
			first start;
			then action make {
				assign cars := addNew(cars, new Car(1));
				assign cars := addNew(cars, new Car(2));
			}
			then action measure {
				assign count := size(cars);
				assign extent := size(all Car);
				assign distinct := not (cars#(1) === cars#(2)) and not (cars#(1) === flagship);
				assign secondN := cars#(2).n;
				assign secondWheels := cars#(2).wheels;
				assign secondAlive := isDuring(cars#(2));
				assign secondIsCar := cars#(2) istype Car;
			}
			then done;
		}
	}
}
