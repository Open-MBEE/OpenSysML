package test {
	// A typed calc usage that redeclares each inherited parameter with a value and
	// inherits its result from the definition's return parameter.
	calc acc : Acceleration {
		in tp = 1.0;
		in tm = 4.0;
	}

	calc def Acceleration {
		in tp : Real;
		in tm : Real;
		return : Real = tp / tm;
	}
}
