// A multi-valued feature is unique unless declared nonunique (KerML 7.3.4.4,
// isUnique defaults true), ordered or not, so a repeat written to it is a
// uniqueness violation the runtime refuses on the write; `nonunique` opts out,
// and a valid ordered feature keeps the order written. Each repeat here arrives
// through a feature reference, so the checker cannot decide it statically (the
// constant-literal case is testdata/passes/unique_values.sysml). The structured
// repeats compare as a Set's members do: a quantity in another unit, an
// enumeration literal and a string.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	enum def Color { red; green; }

	part def Holder {
		attribute one : Integer = 1;
		attribute red : Color = Color::red;
		attribute a : String = "a";
		attribute twoKg :> ISQ::mass = 2 [kg];

		attribute xs : Integer[*] = (one, 1);
		attribute repeats : Integer[*] nonunique = (one, 1);
		attribute inOrder : Integer[*] ordered = (one, 2, 1);
		attribute distinct : Integer[*] ordered = (3, one, 2);
		attribute mixed : Real[*] = (one, 1.0);
		attribute masses :> ISQ::mass [*] = (twoKg, 2000 [g]);
		attribute colors : Color[*] = (red, Color::green, Color::red);
		attribute words : String[*] = (a, "b", "a");
		attribute dynamic : Integer[*] = (distinct, 3);
		attribute dynamicOk : Integer[*] = (distinct, 4);
	}

	part h : Holder;
}
