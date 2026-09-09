// An evaluation is selected only for a value the case picked with `selectOne`:
// a case whose result merely equals an argument of one of its evaluations, and
// whose evaluations compute alike, selects and ties none of them.
package test {
	private import ScalarValues::*;

	part def Ship { attribute cost : Real = 4.0; }
	part ship : Ship;

	analysis def Study {
		subject s : Ship;
		calc flat { in x : Real; return : Real = 1.0; }
		in calc f = flat;
		out total : Real = f(s.cost) + f(2.0);
		return : Real = s.cost;
	}
	analysis study : Study { subject s = ship; }
}
