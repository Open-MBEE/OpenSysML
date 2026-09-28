// A state attribute default evaluated on the exhibited occurrence's
// initialization sees `this` as that occurrence: `same` holds `this == this`
// as true, and the entry action writes it through the context parameter.
package test {
	private import ScalarValues::*;

	part def Plant {
		attribute flag : Boolean = false;

		state def Life {
			in ref context : Plant;
			attribute same : Boolean = this == this;
			entry; then work;
			state work {
				entry action check {
					assign context.flag := same;
				}
			}
		}

		exhibit state life : Life { in ref :>> context = this; }
	}
}
