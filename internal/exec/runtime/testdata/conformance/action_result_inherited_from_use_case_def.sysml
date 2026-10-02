package test {
	use case def UC {
		return n : Integer;
	}
	action def ByUC :> UC {
		first start;
		then action go {
			assign n := 7;
		}
		then done;
	}
}
