package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def BodyLocalResult {
		in n : Integer;
		return : Integer[*] =
			(1..n)->collect {
				in i : Integer;
				private attribute doubled : Integer = i * 2;
				doubled
			};
	}
}
