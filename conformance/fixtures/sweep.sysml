package Sw {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real default = 30.0;
		attribute other : Real default = 7.0;
	}

	calc def Sum {
		in a : Real;
		in b : Real;
		return : Real = a + b;
	}

	calc def Ratio {
		in a : Real;
		in b : Integer;
		return : Real = a / b;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		in limit : Real = 20.0;
		out total : Real = Sum(s.cost, s.other);
		objective affordable {
			require constraint { total <= limit }
		}
	}

	part barge : Ship;
}
