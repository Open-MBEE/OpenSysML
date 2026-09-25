package test {
	calc add {
		in x: Integer;
		in y: Integer;
		x + y
	}
	
	part def Result {
		attribute sum: Integer = add(3, 5);
	}
}
