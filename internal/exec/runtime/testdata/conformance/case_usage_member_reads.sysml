// A case usage's subject, actors and inputs are features of it as its return
// parameter is (SysML v2 §7.22.2, §7.24.2): read through the usage, each answers
// the value the usage binds — on an instantiated owner and at package level alike —
// its objective reads through the case's occurrence, and a subject or actor nothing
// binds is undetermined rather than refused.
package test {
	private import ScalarValues::*;
	private import VerificationCases::*;

	part def Operator { attribute id : Integer = 7; }
	part op : Operator;

	requirement def Positive {
		subject n : Integer;
		require constraint { n > 0 }
	}

	verification def CheckPositive {
		subject n : Integer;
		actor who : Operator;
		in limit : Integer = 10;
		objective { verify requirement pos : Positive { subject :>> n; } }
		return verdict : VerdictKind = PassIf(n > 0 and n <= limit);
	}

	analysis def Scale {
		subject n : Integer;
		actor who : Operator;
		in k : Integer = 2;
		objective fits { require constraint { n * k < 100 } }
		return r : Integer = n * k;
	}

	use case def Ride {
		subject n : Integer;
		actor who : Operator;
	}

	part def Holder {
		attribute value : Integer = 3;
		verification check : CheckPositive { subject n = value; actor who = op; }
		analysis scaled : Scale { subject n = value; actor who = op; }
	}
	part holder : Holder;

	verification checkThree : CheckPositive { subject n = 3; actor who = op; }
	verification checkNothing : CheckPositive;
	analysis scaledFour : Scale { subject n = 4; }
	use case ride : Ride { subject n = 3; actor who = op; }

	attribute ownerSubject = holder.check.n;
	attribute ownerVerdict = holder.check.verdict;
	attribute ownerActorId = holder.check.who.id;
	attribute ownerLimit = holder.check.limit;
	attribute ownerScaledSubject = holder.scaled.n;
	attribute ownerScaledResult = holder.scaled.r;
	attribute ownerObjective = holder.scaled.fits.result;
	attribute packageSubject = checkThree.n;
	attribute packageActorId = checkThree.who.id;
	attribute packageVerdict = checkThree.verdict;
	attribute unboundSubject = checkNothing.n;
	attribute unboundActor = scaledFour.who;
	attribute scaledResult = scaledFour.r;
	attribute useCaseSubject = ride.n;
	attribute useCaseActorId = ride.who.id;
}
