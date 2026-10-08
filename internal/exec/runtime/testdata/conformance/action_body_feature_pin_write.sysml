package test {
	private import ScalarValues::*;

	action def Consumer {
		in value : Integer = 1;
		out result : Integer;
		assign result := value;
	}

	action run {
		attribute source : Integer := 4;
		out attribute result : Integer;
		first start;
		then action consume : Consumer { in value = source; }
		then action update {
			assign source := 8;
			assign result := consume.result;
		}
		then done;
	}
}
