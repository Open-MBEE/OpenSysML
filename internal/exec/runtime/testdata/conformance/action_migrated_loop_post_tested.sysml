// The post-tested loop a LoopNode writes: body before test, an untyped
// multi-valued loop variable carried as a ref.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	action def MigratedLoopPostTested {
		first start then post;
		action post {
			in xIn[0..*] nonunique = (1);
			out xOut[0..*] nonunique;
			private ref x[0..*] nonunique := xIn;
			private attribute ended : Boolean := false;
			first start then iterate;
			then action iterate {
				loop {
					action body {
						first start then grow;
						action grow {
							in v[0..*] nonunique = x;
							out r[0..*] nonunique;
							assign r := including(v, 7);
						}
					}
					then action next {
						assign x := body.grow.r;
					}
					then action test {
						first start then lt;
						action lt {
							in v[0..*] nonunique = x;
							out r : Boolean;
							assign r := size(v) < 3;
						}
					}
					then if not test.lt.r {
						assign ended := true;
					}
				} until ended;
			}
			then action results {
				assign xOut := x;
			}
		}
		first post then final;
		action final terminate;
	}
}
