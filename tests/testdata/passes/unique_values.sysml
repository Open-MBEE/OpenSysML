package UniqueValues {
	private import ScalarValues::*;
	private import Collections::*;

	part def P {
		attribute xs : Integer[*] = (1, 1);
		attribute repeats : Integer[*] nonunique = (1, 1);
		attribute inOrder : Integer[*] ordered = (1, 2, 1);
		attribute distinct : Integer[*] ordered = (3, 1, 2);
		attribute os : OrderedSet { :>> elements = (1, 1, 2); }
		attribute bag : Bag { :>> elements = (1, 1, 2); }
		attribute set : Set { :>> elements = (1, 1, 2); }
		attribute mixed : Real[*] = (1, 1.0);
		attribute words : String[*] = ("a", "b", "a");
		attribute dynamic : Integer[*] = (xs, xs);
	}
}
