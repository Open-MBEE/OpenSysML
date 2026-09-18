calc def AdvancedCalculation {
	in attribute x : Real;
	in attribute y : Real;
	
	// nested calc
	calc inner {
		in a : Real;
		return : Real = a * 2;
	}
	
	// attribute member
	attribute temp : Real;
	
	// return member (required)
	return : Real {
		temp = inner(x);
	}
}
