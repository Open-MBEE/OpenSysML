package test {
	// The usage declares no parameters and no body: both parameters, the default
	// of the second, and the result expression are inherited from the calc
	// definition it is typed by.
	calc scaled : lib::Scale;

	package lib {
		calc def Scale {
			in x: Integer;
			in factor: Integer = 2;
			x * factor
		}
	}
}
