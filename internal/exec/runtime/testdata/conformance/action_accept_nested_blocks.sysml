// An accept node stands wherever a statement does (Actions.sysml:
// `acceptSubactions :> subactions`), so one two blocks deep — in an `if`
// branch inside a `while` body — parks the body's performance until a message
// arrives, and the branch not taken waits for nothing. `then` orders the body's
// statements, which no succession would otherwise order.
package test {
	private import ScalarValues::*;

	action acceptsNested {
		attribute total : Integer = 0;
		attribute i : Integer = 0;
		first start;
		fork split;
		action reader {
			while i < 3 {
				if i < 2 {
					accept n : Integer;
					then assign total := total + n;
				} else {
					if false {
						accept skipped : Integer;
					}
				}
				then assign i := i + 1;
			}
		}
		action gate1;
		action gate2;
		action sender { send 4; send 6; }
		join sync;
		done;
		succession first start then split;
		succession first split then reader;
		succession first split then gate1;
		succession first gate1 then gate2;
		succession first gate2 then sender;
		succession first reader then sync;
		succession first sender then sync;
		succession first sync then done;
	}
}
