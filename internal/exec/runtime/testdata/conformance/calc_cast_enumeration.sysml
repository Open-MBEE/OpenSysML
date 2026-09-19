// An enumeration value is kept by the enumeration that declares it and by every
// type that enumeration specializes, and dropped by a sibling enumeration whose
// values it is not one of (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;

	attribute def Mode;

	enum def Signal :> Mode {
		on;
		off;
	}

	enum def Other :> Mode {
		idle;
	}

	calc def EnumerationCasts {
		out asOwnEnumeration : Signal[0..1] = Signal::on as Signal;
		out asSupertype : Mode[0..1] = Signal::on as Mode;
		out asDataValue : Base::DataValue[0..1] = Signal::on as Base::DataValue;
		out asSibling : Mode[0..1] = Signal::on as Other;
	}

	calc c : EnumerationCasts;
}
