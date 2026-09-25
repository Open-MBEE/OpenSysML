// A calculation body item is an action body item, so a kind-less `x = e;` in a
// calc, constraint or statement body declares a feature with a value
// (SysML.xtext CalculationBodyItem -> NonOccurrenceUsageMember -> DefaultReferenceUsage).
// An assignment is written `assign x := e;` (AssignmentNode).
package DefaultReferenceUsages {
	private import ScalarValues::*;
	private import ISQ::*;

	// A `return` declares its result parameter, so a specialization may follow
	// it with or without a name (SysML.xtext ReturnParameterMember).
	calc def Power {
		in force : Real;
		in speed : Real;
		return :> ISQ::power = force * speed;
	}
	calc def Delta {
		in v0 : Real;
		in v1 : Real;
		return deltaV :> ISQ::speed = v1 - v0;
	}
	calc def Samples {
		in xs : LengthValue[*];
		return :> ISQ::length[*] = xs;
	}

	// The result's identification may open with a short name, with or without a name.
	calc def Named {
		in x : Real;
		return <r> result : Real = x;
	}
	calc def ShortOnly {
		in x : LengthValue;
		return <r> :> ISQ::length = x;
	}
	calc def ShortValued {
		in x : Real;
		return <r> = x;
	}
	calc def ShortBare {
		return <r>;
	}

	// The declaration may open with a multiplicity instead (FeatureSpecializationPart),
	// and a body may follow the identification directly (UsageBody).
	calc def Every {
		in xs : Integer[*];
		return [*] = xs;
	}
	calc def EveryTyped {
		in xs : Integer[*];
		return [*] : Integer = xs;
	}
	calc def EverySubsets {
		in xs : Integer[*];
		return [*] :> xs;
	}
	calc def Documented {
		in n : Integer;
		return r { doc /* the result */ }
	}
	calc def ShortDocumented {
		in n : Integer;
		return <r> { doc /* the result */ }
	}

	// Kind-less members of a calc body are readable locals of its trailing expression.
	calc def Affine {
		in n : Integer;
		twice = n * 2;
		offset := 1;
		twice + offset
	}
	// A body may follow a kind-less member's name directly (DefaultReferenceUsage's UsageBody).
	calc def Bodied {
		in n : Integer;
		twice { doc /* two n */ }
		<t> thrice : Integer { doc /* three n */ }
		n
	}

	part def Tank {
		attribute level : Real;
		assert constraint bounded {
			limit = 10.0;
			floor := 0.0;
			flag { doc /* a bodied local */ }
			level < limit and level > floor
		}
	}

	// A statement body carries the same members; the assignment keeps its keyword.
	calc def Sum {
		in n : Integer;
		attribute total : Integer = 0;
		attribute i : Integer = 1;
		while i <= n {
			doubled = i * 2;
			assign total := total + doubled;
			assign i := i + 1;
		}
		total
	}
}
