package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def Unary { in x : Real; return : Real; }
	calc def Same {
		in calc f { in x : Real; return : Real; }
		in calc g { in x : Real; return : Real; }
		return : Boolean = f == g;
	}
	calc def Maker {
		in k : Real;
		calc scale :> Unary { in :>> x; return : Real = x * k; }
		return : Unary = scale;
	}
	// A nested calc read twice in one run is one function: the two reads are equal,
	// and a sequence of both holds one distinct value.
	calc def SameRun {
		in k : Real;
		calc inner { in x : Real; return : Real = x * k; }
		return : Boolean = Same(inner, inner) and inner == inner
			and includes((inner, inner), inner) and isEmpty(excluding((inner, inner), inner))
			and size(intersection((inner, inner), inner)) == 2;
	}
	// Functions returned by two runs are distinct even when the runs bind the
	// closed-over name alike: each run's function is its own.
	calc def AcrossRuns {
		in calc once :> Unary = Maker(2.0);
		return : Boolean = Maker(2.0) != Maker(2.0) and Maker(2.0) != Maker(3.0)
			and once == once and excludes((Maker(2.0), Maker(3.0)), once)
			and size(excluding((once, Maker(2.0), once), once)) == 1;
	}
	// Within one performance, a read taken before the body rebinds a closed-over
	// name and a read taken after are the same function.
	action def Rebinding {
		out sameRun : Boolean = SameRun(3.0);
		out acrossRuns : Boolean = AcrossRuns();
		out before : Boolean;
		out after : Boolean;
		attribute k : Real = 2.0;
		calc byK { in x : Real; return : Real = x * k; }
		attribute held = byK;

		first start;
		then action same { assign before := held == byK; }
		then action rebind { assign k := 3.0; }
		then action changed { assign after := held == byK; }
		then done;
	}
	action rebinding : Rebinding;
}
