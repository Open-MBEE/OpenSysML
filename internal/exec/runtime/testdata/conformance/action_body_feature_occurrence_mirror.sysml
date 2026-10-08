package test {
	private import ScalarValues::*;

	part def Host {
		attribute mirrored : Integer := 0;
		attribute result : Integer := 0;

		action def Bind {
			in ref context : Host[1];
			attribute source : Integer := 0;
			attribute live : Integer = source * 2;

			first start;
			then action observe { assign context.mirrored := this.live; }
			then action write { assign source := 5; }
			then action read {
				assign context.mirrored := this.live;
				assign context.result := live;
			}
			then done;
		}

		perform action bind : Bind { in ref :>> context = this; }
	}
}
