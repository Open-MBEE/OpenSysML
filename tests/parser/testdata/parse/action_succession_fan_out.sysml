action def FanOut {
	attribute armed : Integer = 0;

	first start;
	then action a;
	action b;
	action c;
	action d;

	succession first a then b;
	succession first a if armed > 0 then c;
	first a then d;
	then if armed == 0 { assign armed := 1; }
	then b;
	then c;
}
