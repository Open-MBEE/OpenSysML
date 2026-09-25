package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def BodyForwardReference {
		in n : Integer;
		return : Integer[*] =
			(1..n)->collect {
				in i : Integer;
				private attribute computed : Integer = laterValue + 1;
				private attribute laterValue : Integer = i;
				computed
			};
	}
}
