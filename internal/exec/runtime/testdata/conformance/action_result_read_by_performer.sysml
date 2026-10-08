package test {
	calc def Twice {
		in x : Integer;
		return y : Integer;
	}
	action def Doubler :> Twice {
		first start;
		then action go {
			assign y := x * 2;
		}
		then done;
	}
	action def Host {
		out got : Integer;
		first start;
		then action d : Doubler {
			in x = 21;
		}
		then action read {
			assign got := d.y;
		}
		then done;
	}
}
