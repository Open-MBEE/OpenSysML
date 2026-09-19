package test {
	// `and`/`or` short-circuit and `xor`/`implies` are evaluated at invocation
	// time, so each holds for values only known from the parameters.
	calc classify {
		in a: Boolean;
		in b: Boolean;
		attribute both : Boolean = a and b;
		attribute either : Boolean = a or b;
		attribute differ : Boolean = a xor b;
		attribute follows : Boolean = a implies b;
		if both {
			return : String = "both";
		}
		if differ and either {
			return : String = "one";
		}
		if follows {
			return : String = "neither";
		}
		return : String = "unreachable";
	}
}
