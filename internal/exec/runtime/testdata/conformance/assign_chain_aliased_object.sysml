// Two features holding one occurrence see one write: `twin` references the same
// cell `cell` names, so writing through either chain writes that one object.
package test {
	private import ScalarValues::*;

	part def Cell {
		attribute mark : Integer = 0;
	}

	part def Rig {
		part cell : Cell;
		ref part twin : Cell = cell;

		exhibit state run {
			entry; then marking;

			state marking {
				entry action set {
					assign twin.mark := 7;
				}
			}
		}
	}
}
