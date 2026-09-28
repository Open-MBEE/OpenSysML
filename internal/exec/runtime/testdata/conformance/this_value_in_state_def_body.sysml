// `this` in the entry action of a state definition nested in a part
// definition is the definition's own occurrence: the comparison runs, and the
// write it makes through the context parameter lands on the exhibiting object.
package test {
	private import ScalarValues::*;

	part def Plant {
		attribute flag : Boolean = false;

		state def Life {
			in ref context : Plant;
			entry; then work;
			state work {
				entry action check {
					assign context.flag := this == this;
				}
			}
		}

		exhibit state life : Life { in ref :>> context = this; }
	}
}
