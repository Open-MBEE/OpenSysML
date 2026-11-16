// A guarded succession in a calculation body (GuardedSuccession, an
// ActionBodyItem): `bonus` is performed only when the guard holds.
package test {
	private import ScalarValues::*;

	calc guarded {
		in n : Integer;
		attribute x : Integer := 0;
		action base { assign x := 1; }
		action bonus { assign x := x + 10; }
		first base if n > 0 then bonus;
		return : Integer = x;
	}
}
