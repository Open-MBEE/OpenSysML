package test {
	private import ScalarValues::*;

	action def Foo {
		in ref context : Base2[1];
		out attribute c : Integer = 0;
		assign context.hits := context.hits + 1;
		assign c := c + 1;
	}

	action def Base2 {
		attribute hits : Integer = 0;
		action a : Foo { in ref :>> context = this; }
	}

	action def Narrow2 :> Base2 { action :>> a[2]; }
}
