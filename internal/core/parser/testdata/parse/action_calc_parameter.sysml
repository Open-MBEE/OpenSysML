calc def Unary { in v : Real; return : Real; }
action def Apply {
	in calc f { in v : Real; return : Real; }
	in calc g : Unary;
	in a : Real;
	out y : Real;
	action step { assign y := f(a) + g(a); }
}
action apply : Apply {
	in calc h : Unary;
}
