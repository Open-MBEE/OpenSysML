package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	part def Holder {
		attribute shared : Integer = 7;
		attribute values : Integer[*] = (1, 2, 3);
		attribute doubled : Integer[*] =
			values->collect {
				in i : Integer;
				private attribute shared : Integer = i * 2;
				shared
			};
	}
}
