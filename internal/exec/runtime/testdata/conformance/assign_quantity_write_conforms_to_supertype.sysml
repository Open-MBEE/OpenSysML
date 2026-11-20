// A quantity is a ScalarQuantityValue and so a NumericalValue and a
// TensorQuantityValue: features typed by those hold it, and one typed by a
// quantity kind of its dimension holds it in that kind's unit.
package test {
	action travel {
		attribute numerical : ScalarValues::NumericalValue = 0.0;
		attribute tensor : Quantities::TensorQuantityValue = 0.0 [SI::m];
		attribute speed : ISQ::SpeedValue = 0.0 [SI::m / SI::s];
		attribute measured;
		attribute quick : ScalarValues::Boolean = false;

		first start;

		action compute {
			assign measured := 10.0 [SI::m] / 2.0 [SI::s];
			then assign numerical := measured;
			then assign tensor := measured;
			then assign speed := measured;
		}

		action check {
			assign quick := speed >= 18.0 [SI::km/SI::h] and numerical == tensor;
		}

		done;

		succession first start then compute;
		succession first compute then check;
		succession first check then done;
	}
}
