// An unvalued `out result` of a state definition a part exhibits is read when
// the machine starts and holds no value; the object standing in for it is held
// by the part's graph and carried by a held image of the part.
package test {
	private import ScalarValues::Boolean;
	state def Checks {
		out result : Boolean[1];
		entry; then idle;
		state idle;
	}
	part def Checker { exhibit state checks : Checks; }
	part checker : Checker;
}
