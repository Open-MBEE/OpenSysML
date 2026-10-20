// An input nothing supplies — no argument, no default, no same-named feature of
// the object — stays undetermined when read through the chain, for a calc usage
// (which reads as a function value, computing nothing) and for a constraint usage
// alike; an input a default supplies beside it reads the default. A parameter of an
// action usage nothing has invoked is bound at invocation (SysML v2 §7.16), so it reads
// undetermined on the instance as at model level, while a body attribute reads its value.
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
		attribute underLimit = rig.under.limit;
		attribute underSlack = rig.under.slack;
		attribute actIn = rig.act.i;
		attribute actOut = rig.act.o;
		attribute actK = rig.act.k;
	}
}
