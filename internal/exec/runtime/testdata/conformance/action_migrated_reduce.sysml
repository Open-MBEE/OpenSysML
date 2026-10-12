// The form a migrated ReduceAction writes: the result is the collection
// folded by a body calling the reducer's function pairwise.
package test {
	private import ScalarValues::*;

	action def Reducing {
		in feed : Integer[0..*] = (1, 2, 3);

		first start;

		action reduce {
			in collection : Integer[0..*];
			out result : Integer = collection->ControlFunctions::reduce {in x; in y; IntegerFunctions::'+'(x, y)};
		}
		action final terminate;

		first start then reduce;
		first reduce then final;

		bind reduce.collection = feed;
	}
}
