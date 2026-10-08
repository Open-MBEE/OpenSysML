// The loop a LoopNode writes when the LoopNode owns a variable: the variable
// is declared once at loop level, so a write in the setup and each add in the
// body reach the read the test makes on the next pass.
package test {
	private import ScalarValues::*;

	action def MigratedLoopOwnedVariable {
		first start then acc;
		action acc {
			in iIn : Integer = 0;
			out iOut : Integer;
			private attribute i : Integer := iIn;
			private attribute total : Integer;
			private attribute ended : Boolean := false;
			first start then setup;
			action setup {
				first start then init;
				action init {
					assign total := 0;
				}
			}
			then action iterate {
				loop {
					action test {
						first start then readTotal;
						action readTotal {
							out r : Integer = total;
						}
						first readTotal then le;
						action le {
							in x : Integer;
							out r : Boolean;
							assign r := x < 6;
						}
						flow readTotal.r to le.x;
					}
					then if not test.le.r {
						assign ended := true;
					} else {
						action body {
							first start then add;
							action add {
								in x : Integer = i;
								out r : Integer;
								assign total := total + x;
								assign r := x + 1;
							}
						}
						then action next {
							assign i := body.add.r;
						}
					}
				} until ended;
			}
			then action results {
				assign iOut := i;
			}
		}
		first acc then final;
		action final terminate;
	}
}
