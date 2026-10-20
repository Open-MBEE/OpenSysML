// A case usage's subject, actors and inputs are features of it as its return
// parameter is (SysML v2 §7.22.2, §7.24.2): read through the usage, each answers
// the value the usage binds — on an instantiated owner and at package level alike —
// its objective reads through the case's occurrence, and a subject or actor nothing
// binds is undetermined rather than refused. The same read answers a plain calc
// usage's input (§7.19.2) and a case body's own locals and parts.
package test {
	private import ScalarValues::*;
	private import VerificationCases::*;

	part def Operator { attribute id : Integer default = 7; }
	part op : Operator;
	part op2 : Operator { attribute :>> id = 9; }

	requirement def Positive {
		subject n : Integer;
		require constraint { n > 0 }
	}

	verification def CheckPositive {
		subject n : Integer;
		actor who : Operator;
		in limit : Integer = 10;
		in open : Integer;
		in vals : Integer[0..*];
		actor many : Operator[0..*];
		attribute depth : Integer = n + 1;
		part gear : Operator { attribute w : Integer = n * 2; }
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

	calc def Twice {
		in x : Integer;
		return : Integer = x * 2;
	}

	verification def Crewed {
		subject n : Integer;
		actor pilot : Operator;
		attribute pilotId : Integer = pilot.id;
		return verdict : VerdictKind = PassIf(n > 0 and pilot.id > 0);
	}

	use case def Ride {
		subject n : Integer;
		actor who : Operator;
	}

	part def Holder {
		attribute value : Integer default = 3;
		verification check : CheckPositive { subject n = value; actor who = op; }
		analysis scaled : Scale { subject n = value; actor who = op; }
		calc twice : Twice { in x = value; }
	}
	part holder : Holder;
	part other : Holder { :>> value = 5; }

	verification checkThree : CheckPositive { subject n = 3; actor who = op; }
	verification checkMany : CheckPositive { subject n = 3; actor many = (op, op2); in :>> open = 1; }
	calc twoTimes : Twice { in x = 2; }
	verification checkNothing : CheckPositive;
	verification crewed : Crewed { subject n = 3; actor crew = op2; }
	analysis scaledFour : Scale { subject n = 4; }
	use case ride : Ride { subject n = 3; actor who = op; }

	attribute ownerSubject = holder.check.n;
	attribute ownerVerdict = holder.check.verdict;
	attribute ownerActorId = holder.check.who.id;
	attribute ownerLimit = holder.check.limit;
	attribute ownerScaledSubject = holder.scaled.n;
	attribute ownerScaledResult = holder.scaled.r;
	attribute ownerObjective = holder.scaled.fits.result;
	attribute ownerGearWidth = holder.check.gear.w;
	attribute otherSubject = other.check.n;
	attribute otherGearWidth = other.check.gear.w;
	attribute otherScaledResult = other.scaled.r;
	attribute gearsDistinct = holder.check.gear != other.check.gear;
	attribute renamedActorId = crewed.crew.id;
	attribute renamedActorBody = crewed.pilotId;
	attribute renamedActorVerdict = crewed.verdict;
	attribute packageSubject = checkThree.n;
	attribute packageActorId = checkThree.who.id;
	attribute packageVerdict = checkThree.verdict;
	attribute unboundSubject = checkNothing.n;
	attribute unboundActor = scaledFour.who;
	attribute scaledResult = scaledFour.r;
	attribute manyActors = checkMany.many;
	attribute manyVerdict = checkMany.verdict;
	attribute suppliedOpen = checkMany.open;
	attribute unsuppliedOpen = checkThree.open;
	attribute unsuppliedVals = checkThree.vals;
	attribute unboundMany = checkThree.many;
	attribute bodyLocal = checkThree.depth;
	attribute bodyPartId = checkThree.gear.id;
	attribute calcInput = twoTimes.x;
	attribute ownerCalcInput = holder.twice.x;
	attribute useCaseSubject = ride.n;
	attribute useCaseActorId = ride.who.id;
}
