// An input nothing supplies — no argument, no default, no same-named feature of
// the object — stays undetermined when read through the chain, for a calc usage
// (which reads as a function value, computing nothing, so its `result` is undetermined too) and for a constraint usage
// alike; an input a default supplies beside it reads the default. A parameter of an
// action usage nothing has invoked is bound at invocation (SysML v2 §7.17.2), so it reads
// undetermined on the instance as at model level, while a body attribute reads its value.
// A constraint usage whose input nothing binds reads as an undetermined verdict, itself
// and through `result`, rather than failing the read. An input nothing supplies stays
// undetermined whatever lower bound it declares (`[1]` as unstated), and its count is
// the effective range of the parameter for a calc and a constraint alike; a case usage
// read as a feature leaves such an input open too and, like a calc, runs no body on it:
// its result, verdict and body locals read undetermined naming the input.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::size;

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
		calc scaledOne {
			in w : Real[1];
			in k : Real = 2.0;
			attribute s : Real = 5.0;
			return : Real = w * k;
		}
		// A default reading an input nothing supplies is open with it: the run binds the
		// input undetermined, so `v` and the result read undetermined too, never a null.
		calc chained { in w : Real[1]; in v : Real = w + 1.0; return : Real = v; }
		constraint overs { in limit : Real[1]; in slack : Real = limit - 1.0; slack > 0.0 }
		constraint underOne {
			in limit : Real[1];
			limit > 1.0
		}
		analysis def Rated {
			subject s : Real;
			in n : Real[1];
			return : Real = s * 2.0;
		}
		analysis rated : Rated {
			subject s = 3.0;
		}
		analysis def Scored {
			subject s : Real;
			in n : Real[1];
			attribute loc : Real = n + 1.0;
			return : Real = s * n;
		}
		analysis scored : Scored {
			subject s = 3.0;
		}
		verification def Checked {
			subject s : Real;
			in n : Real[1];
			return verdict : VerificationCases::VerdictKind = VerificationCases::PassIf(s > n);
		}
		verification checked : Checked {
			subject s = 3.0;
		}
		action act {
			in i : Integer;
			out o : Integer;
			attribute k : Integer = 3;
		}
	}
	analysis pkgScored : Rig::Scored {
		subject s = 3.0;
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
		attribute scaledWCount = size(rig.scaled.w);
		attribute underLimitCount = size(rig.under.limit);
		attribute scaledOneW = rig.scaledOne.w;
		attribute scaledOneS = rig.scaledOne.s;
		attribute scaledOneResult = rig.scaledOne.result;
		attribute chainedV = rig.chained.v;
		attribute chainedResult = rig.chained.result;
		attribute oversSlack = rig.overs.slack;
		attribute oversVerdict = rig.overs;
		attribute underOneLimit = rig.underOne.limit;
		attribute underOneVerdict = rig.underOne;
		attribute underOneResult = rig.underOne.result;
		attribute ratedN = rig.rated.n;
		attribute ratedResult = rig.rated.result;
		attribute scoredN = rig.scored.n;
		attribute scoredLoc = rig.scored.loc;
		attribute scoredResult = rig.scored.result;
		attribute checkedVerdict = rig.checked.verdict;
		attribute pkgScoredResult = pkgScored.result;
		attribute actIn = rig.act.i;
		attribute actOut = rig.act.o;
		attribute actK = rig.act.k;
	}
}
