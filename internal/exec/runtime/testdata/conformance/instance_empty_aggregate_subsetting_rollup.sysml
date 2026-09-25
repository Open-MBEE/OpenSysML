package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;
	private import NumericalFunctions::*;
	private import SequenceFunctions::size;

	part def MassedComponent {
		part subcomponents : MassedComponent [*] default null;
		attribute mass :> ISQ::mass;
		attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);
	}
	part def Leaf :> MassedComponent {
		attribute :>> mass = 100 [kg];
	}
	part def Stack :> MassedComponent {
		attribute :>> mass = 10 [kg];
		part a : Leaf :> subcomponents;
		part b : Leaf subsets subcomponents;
	}
	part def Tower :> MassedComponent {
		attribute :>> mass = 1 [kg];
		part s : Stack :> subcomponents;
		part c : Leaf :> subcomponents;
	}
	part def Loop {
		part xs : Loop [*] :> ys default null;
		part ys : Loop [*] :> xs default null;
	}
	part def Rig {
		part leaf : Leaf;
		part stack : Stack;
		part tower : Tower;
		part loop : Loop;
		attribute lengths : LengthValue [*];
		attribute counts : Integer [*];
		attribute emptySum : MassValue = sum(leaf.subcomponents.totalMass);
		attribute emptyLengthSum : LengthValue = lengths->sum();
		attribute emptyProduct = product(lengths);
		attribute emptyCount = size(lengths);
		attribute otherEmptyKind = 10 [kg] + sum(lengths);
		attribute emptyCountSum = 10 [kg] + sum(counts);
	}
}
