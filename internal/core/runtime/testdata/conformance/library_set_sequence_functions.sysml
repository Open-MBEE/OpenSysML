// Every SequenceFunctions result is declared `ordered nonunique`
// (SequenceFunctions.kerml: union, intersection, including, excluding), so a
// set given to one is read as the sequence of its members in canonical order
// and the result is a sequence, repeats and all — never a set.
package test {
	private import Collections::*;
	private import SequenceFunctions::*;

	attribute s : Set { :>> elements = (3, 1, 2, 2); }
	attribute t : Set { :>> elements = (2, 4); }

	attribute both : Integer[0..*] ordered nonunique = union(s.elements, t.elements);
	attribute common : Integer[0..*] ordered nonunique = intersection(s.elements, t.elements);
	attribute more : Integer[0..*] ordered nonunique = including(s.elements, 2);
	attribute fewer : Integer[0..*] ordered nonunique = excluding(s.elements, 2);
	attribute joined : Integer[0..*] ordered nonunique = (s.elements, s.elements);
}
