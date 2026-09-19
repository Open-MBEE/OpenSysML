// A member of an Array specialization is not confused with Array's own features
// because of its name: `order` is TensorQuantityValue's redefinition of rank,
// not Array's, so a model's own `order` answers its own value.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute def OrderedGrid :> Array {
		attribute order : Integer = 7;
	}

	attribute grid : OrderedGrid {
		:>> dimensions = (2, 2);
		:>> elements = (1, 2, 3, 4);
	}

	calc def shape {
		return : Integer[*] = (grid.order, grid.rank, grid.flattenedSize);
	}
}
