package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def BodyBareResult {
		attribute k : Integer = 99;
		attribute values : Integer[*] = (1, 2, 3);
		values->collect {
			in i : Integer;
			private attribute k : Integer = i * 2;
			k
		}
	}
}
