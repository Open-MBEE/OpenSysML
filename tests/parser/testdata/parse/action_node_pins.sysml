package test {
	private import ScalarValues::*;

	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;
	}

	action outer {
		attribute x : Integer = 5;
		out attribute total : Integer;

		bind add.a = x;

		first start;
		then action add : Adder { in b = 4; }
		then action scaled = Adder(3, 4);
		then action named = Adder(b = 4, a = 3);
		then action fin { in n : Integer; assign total := n + scaled; }
		then done;

		flow add.sum to fin.n;
	}
}
