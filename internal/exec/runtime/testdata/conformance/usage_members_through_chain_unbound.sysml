// An input nothing supplies — no argument, no default, no same-named feature of
// the object — stays undetermined when read through the chain, for a calc usage
// (which reads as a function value, computing nothing, so its `result` is undetermined too) and for a constraint usage
// alike; an input a default supplies beside it reads the default. A parameter of an
// action usage nothing has invoked is bound at invocation (SysML v2 §7.17.2), so it reads
// undetermined on the instance as at model level, while a body attribute reads its value.
// A constraint usage whose input nothing binds reads as an undetermined verdict, itself
// and through `result`, rather than failing the read.
package test {
	private import ScalarValues::*;

	part def Rig {
		calc scaled {
			in w : Real;
			in k : Real = 2.0;
			return : Real = w * k;
		}
		constraint under {
			in limit : Real;
			in slack : Real = 1.0;
			limit > slack
		}
		action act {
			in i : Integer;
			out o : Integer;
			attribute k : Integer = 3;
		}
	}
	part def Bench {
		part rig : Rig;
		attribute scaledW = rig.scaled.w;
		attribute scaledK = rig.scaled.k;
		attribute scaledResult = rig.scaled.result;
		attribute underLimit = rig.under.limit;
		attribute underSlack = rig.under.slack;
		attribute underVerdict = rig.under;
		attribute underResult = rig.under.result;
		attribute actIn = rig.act.i;
		attribute actOut = rig.act.o;
		attribute actK = rig.act.k;
	}
}
