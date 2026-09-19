// An action a part performs creates, adds and destroys occurrences during its
// own performance: a part the body first reaches inside `create` begins during
// the call, `isDuring` answers whether the call happens during the argument's
// lifetime, `addNew` appends the created part to a group, and nothing happens
// during a destroyed part any more.
package test {
	private import ScalarValues::*;
	private import OccurrenceFunctions::*;
	private import SequenceFunctions::size;

	part def Widget { attribute n : Integer = 1; }

	part def Robot {
		part arm : Widget;
		part spares : Widget[0..*];
		attribute createdIsFresh : Boolean;
		attribute armDuring : Boolean;
		attribute goDuring : Boolean;
		attribute freshDuringGo : Boolean;
		attribute freshDuringWork : Boolean;
		attribute spareCount : Integer;
		attribute armAfter : Boolean;

		perform action go {
			part fresh : Widget;
			part spare : Widget;
			first start;
			then action begin {
				assign createdIsFresh := create(fresh) === fresh;
				assign freshDuringGo := isDuring(fresh);
			}
			then action work {
				assign armDuring := isDuring(arm);
				assign goDuring := isDuring(go);
				assign freshDuringWork := isDuring(fresh);
				assign spares := addNew(spares, spare);
				assign spareCount := size(spares);
				assign arm := destroy(arm);
				assign armAfter := isDuring(arm);
			}
			then done;
		}
	}
}
