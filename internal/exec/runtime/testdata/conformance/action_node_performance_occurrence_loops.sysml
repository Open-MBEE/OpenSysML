// Loop performances and node performances hold one another: a loop a named node reaches
// is held under that node's occurrence (`run.stage.loops`), and a named node an iteration
// performs is held under that iteration's `body`, named body action or not, with the
// loops it reaches in turn under it (KerML 1.1 `Performances::subperformances`, §7.3.4.6).
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part plant {
		attribute namedBodyValues : Integer[0..*] = run.stage.loops#(1).body.inc.v;
		attribute stageLoops : Integer[1] = size(run.stage.loops);
		attribute unnamedBodyValues : Integer[0..*] = run.loops#(1).body.add.w;
		attribute runLoops : Integer[1] = size(run.loops);
		attribute innerValues : Integer[0..*] = run.loops#(1).body.add.loops.body.deep.x;
		perform action run {
			attribute n : Integer[1] = 0;
			attribute m : Integer[1] = 0;
			first start;
			then action stage {
				first start;
				then loop action body {
					first start;
					then action inc {
						out v : Integer[1];
						first start;
						then assign v := n + 1;
						then assign n := v;
						then done;
					}
					then done;
				} until n >= 3;
				then done;
			}
			then while m < 2 {
				action add {
					out w : Integer[1];
					attribute j : Integer[1] = 0;
					first start;
					then assign w := m + 10;
					then assign m := m + 1;
					then loop action {
						action deep {
							out x : Integer[1];
							first start;
							then assign x := w * 100 + j;
							then assign j := j + 1;
							then done;
						}
					} until j >= 2;
					then done;
				}
			}
			then done;
		}
	}
}
