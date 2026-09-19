// An object typed directly by an Array specialization fixing its dimensions
// (`Grid::dimensions = (2, 2)`) is validated against them: three elements do not
// fill a 2×2 grid, and the read is a multiplicity violation, not an Array.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute def Grid :> Array {
		attribute :>> dimensions = (2, 2);
	}

	attribute g : Grid {
		attribute :>> elements = (1, 2, 3);
	}

	calc def size {
		return : Natural = g.flattenedSize;
	}
}
