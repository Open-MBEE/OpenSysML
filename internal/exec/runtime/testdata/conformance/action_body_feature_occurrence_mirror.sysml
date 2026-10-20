// A body feature's value is mirrored on the performance the performer holds:
// `context.bind.live`, read through the performing Host's `bind`, follows the
// body's `source` as the body's own name `live` does. `this` in the body is the
// Host performing it (Parts.sysml `ownedActions::this = that as Part`), so the
// performance is reached through the performer, not through `this`.
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
			then action observe { assign context.mirrored := context.bind.live; }
			then action write { assign source := 5; }
			then action read {
				assign context.mirrored := context.bind.live;
				assign context.result := live;
			}
			then done;
		}

		perform action bind : Bind { in ref :>> context = this; }
	}
}
