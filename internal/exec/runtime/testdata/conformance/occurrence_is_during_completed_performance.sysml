// A performed action is an occurrence from its start to its completion: a call
// in its body happens during it, one after it has completed does not.
package test {
	private import ScalarValues::*;
	private import OccurrenceFunctions::*;

	part def Robot {
		attribute goAfter : Boolean = isDuring(go);
		attribute goWhile : Boolean;
		perform action go {
			first start;
			then action work {
				assign goWhile := isDuring(go);
			}
			then done;
		}
	}
}
