package test {
	private import ScalarValues::*;
	action seq {
		attribute result : Integer = 0;
		action source {
			attribute pin : Integer = 0;
			assign result := 7;
		}
		action target { assign result := result + 5; }
		succession first start then source;
		succession first source.pin then target;
		succession first target then done;
	}
}
