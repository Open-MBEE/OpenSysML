private import ScalarValues::*;

package test {
	metadata def Safety;

	part def Vehicle;
	part def Car :> Vehicle;
	#Safety part def Belt;

	attribute n3 : Integer = 3;
	attribute r25 : Real = 2.5;
	attribute seqInt : Integer[*] = (1, 2, 3);
	part car : Car;
	#Safety part belt : Belt;

	// `x @ T` on a value is `x istype T`: it holds for T and every type the value's
	// type specializes. `x @ Metadata` still tests x's annotations, `@@` its metaclass.
	calc classify {
		return : Boolean[*] nonunique = (
			n3 @ Integer,
			n3 @ Real,
			n3 @ String,
			n3 hastype Integer,
			n3 hastype Real,
			r25 @ Real,
			r25 @ Integer,
			3 @ Integer,
			seqInt @ Integer,
			car @ Car,
			car @ Vehicle,
			car @ Belt,
			belt @ Belt,
			belt @ Safety,
			car @ Safety,
			belt @@ SysML::PartUsage,
			Belt @ Safety
		);
	}
}
