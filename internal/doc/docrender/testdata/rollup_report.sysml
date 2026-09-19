package Rollup {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;
	private import NumericalFunctions::*;

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
	part hangar {
		part leaf : Leaf;
		part stack : Stack;
		part tower : Tower;
	}

	calc def Masses :> Query {
		in root : Element = hangar;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass", "totalMass")
		)
	}

	part def MassReport :> Document {
		attribute redefines title = "Rolled-Up Masses";

		part all : Table {
			attribute redefines caption = "Own mass and total mass";
			calc rows : Masses;
		}

		part totals : Definitions {
			attribute redefines term = "name";
			attribute redefines description = "totalMass";
			calc rows : Masses;
		}
	}
}
