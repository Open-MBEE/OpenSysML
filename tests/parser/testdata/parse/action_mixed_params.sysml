action def MixedParamsAction {
	in attribute input1 : Real;
	in attribute input2[0..*] : Integer;
	out attribute output : String;
	inout attribute counter : Integer;
	
	action step1;
	action step2 {
		in x : Real;
		out y : String;
	}
	action step3;
}
