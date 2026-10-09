package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Box {
		attribute values : Integer[0..*] ordered nonunique;
	}

	action write {
		in object : Box;
		in value : Integer;
		assign object.values := SequenceFunctions::including(object.values, value);
		out result : Box = object;
	}
}
