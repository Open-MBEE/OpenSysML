package test {
	calc def Answer {
		return r : Integer;
	}
	action def Compute :> Answer {
		first start;
		then action go {
			assign r := 42;
		}
		then done;
	}
}
