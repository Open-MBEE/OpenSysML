// A body-local declaration's value is a value of the feature it declares, so it
// answers to the declared type, multiplicity and uniqueness at run time exactly
// as a parameter or `return` does (KerML 1.0 §7.3.4: "the values of a feature
// are instances of its types"). Level's extent is {1, 3}: 3 is held as
// Level::high, 2 is not a Level and is refused before the body goes on. A local
// declaring no type holds anything; one stating no multiplicity holds the count
// its initializer has, as a namespace-level declaration does.
package test {
	private import ScalarValues::*;

	enum def Level :> Integer { low = 1; high = 3; }

	calc def BodyOnly { in n : Integer; attribute l : Level = n; return : Integer = l + 0; }
	calc def HeldAs { in n : Integer; attribute l : Level = n; return : Level = l; }
	calc def HeldIsHigh { in n : Integer; attribute l : Level = n; return : Boolean = l === Level::high; }
	calc def UntypedIsHigh { in n : Integer; attribute u = n; return : Boolean = u === Level::high; }
	calc def TwoOf { in n : Integer; attribute xs : Integer[2] = (n, n + 1, n + 2); return : Integer = SequenceFunctions::size(xs); }
	calc def Distinct { in n : Integer; attribute xs : Integer[*] = (n, n + 1, n); return : Integer = SequenceFunctions::size(xs); }
	calc def Untyped { in n : Integer; attribute u = n; return : Integer = u; }
	calc def Unstated { in n : Integer; attribute xs = (n, n + 1, n + 2); return : Integer = SequenceFunctions::size(xs); }
	calc def InBlock {
		in n : Integer;
		attribute r : Integer = 0;
		if n > 0 { attribute l : Level = n; assign r := l; }
		return : Integer = r;
	}
	calc def InCollection {
		in n : Integer;
		return : Integer = (n, n)->ControlFunctions::collect { in i : Integer; attribute l : Level = i; l + 0 }->SequenceFunctions::size();
	}

	calc def Probe {
		in n : Integer;
		out accepted : Integer = BodyOnly(3);
		out heldAs : Level = HeldAs(3);
		out heldIsHigh : Boolean = HeldIsHigh(3);
		out untypedIsHigh : Boolean = UntypedIsHigh(3);
		out untyped : Integer = Untyped(2);
		out unstated : Integer = Unstated(1);
		out inBlockAccepted : Integer = InBlock(3);
		out inCollectionAccepted : Integer = InCollection(1);
		return : Integer = n;
	}
	calc probe : Probe { in n = 0; }

	attribute refused : Integer = BodyOnly(2);
	attribute tooMany : Integer = TwoOf(1);
	attribute repeated : Integer = Distinct(1);
	attribute inBlockRefused : Integer = InBlock(2);
	attribute inCollectionRefused : Integer = InCollection(2);
}
