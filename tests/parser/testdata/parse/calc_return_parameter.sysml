// A lone name after 'return' declares the return parameter of the calc,
// which a specializing usage inherits and members can then reference.
calc def Acceleration {
	in tp : Real;
	in tm : Real;
	return : Real = tp / tm;
}

calc acc : Acceleration {
	in tp = 1;
	in tm = 2;
	return a;
}

calc bounded : Acceleration {
	return sampled[0..1];
}

calc computed {
	in x : Real;
	x * 2
}
