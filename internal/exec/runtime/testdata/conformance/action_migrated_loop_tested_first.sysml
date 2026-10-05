// The tested-first loop a LoopNode writes: setup once, test before body, the
// loop variables and an ended flag re-initialised per performance.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	action def MigratedLoopTestedFirst {
		first start then 'loop';
		action 'loop' {
			in iIn : Integer = 0;
			in sIn : Integer = 0;
			out iOut : Integer;
			out sOut : Integer;
			private attribute i : Integer := iIn;
			private attribute s : Integer := sIn;
			private attribute ended : Boolean := false;
			first start then setup;
			action setup {
				first start then zero;
				action zero {
					out r : Integer;
					assign r := 0;
				}
			}
			then action iterate {
				loop {
					action test {
						first start then le;
						action le {
							in x : Integer = i;
							in y : Integer = s;
							in z : Integer = setup.zero.r;
							out r : Boolean;
							out k : Integer;
							assign r := x <= y;
							assign k := 10 * x;
						}
					}
					then if not test.le.r {
						assign ended := true;
					} else {
						action body {
							first start then plus;
							action plus {
								in x : Integer = s;
								in y : Integer = test.le.k;
								out r : Integer;
								assign r := x + y;
							}
							first plus then inc;
							action inc {
								in x : Integer = i;
								out r : Integer;
								assign r := x + 1;
							}
						}
						then action next {
							assign i := body.inc.r;
							then assign s := body.plus.r;
						}
					}
				} until ended;
			}
			then action results {
				assign iOut := i;
				then assign sOut := s;
			}
		}
		first 'loop' then final;
		action final terminate;
	}
}
