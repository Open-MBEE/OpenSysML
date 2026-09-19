package test {
	private import ScalarValues::*;

	// A lower bound of one admits no empty write: emptiness is judged by the
	// target's multiplicity rather than waved through.
	part def Source {
		attribute raw[0..1] = ();
	}

	part def Rig {
		attribute readings : Real[1] = 0.5;
		part source : Source;

		exhibit state run {
			entry; then go;

			state go {
				entry action clear {
					assign readings := source.raw;
				}
			}
		}
	}
}
