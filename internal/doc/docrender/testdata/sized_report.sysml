package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}

	part telescope {
		part optics : Subsystem {
			doc /* The primary mirror assembly. */
			attribute redefines mass = 8.5;
			part cell : Subsystem {
				doc /* Holds the mirror. */
				attribute redefines mass = 2.0;
			}
		}
		part mount : Subsystem {
			doc /* Points the telescope. */
			attribute redefines mass = 15.0;
		}
	}

	part def Reading {
		attribute a : Integer;
		attribute b : Integer;
		attribute c : Integer;
		attribute d : Integer;
		attribute e : Integer;
		attribute f : Integer;
		attribute g : Integer;
		attribute h : Integer;
		attribute i : Integer;
		attribute j : Integer;
		attribute k : Integer;
		attribute l : Integer;
		attribute m : Integer;
		attribute n : Integer;
		attribute o : Integer;
		attribute p : Integer;
		attribute q : Integer;
		attribute r : Integer;
		attribute s : Integer;
	}

	part sensors {
		part sample : Reading {
			attribute redefines a = 1;
			attribute redefines b = 2;
			attribute redefines c = 3;
			attribute redefines d = 4;
			attribute redefines e = 5;
			attribute redefines f = 6;
			attribute redefines g = 7;
			attribute redefines h = 8;
			attribute redefines i = 9;
			attribute redefines j = 10;
			attribute redefines k = 11;
			attribute redefines l = 12;
			attribute redefines m = 13;
			attribute redefines n = 14;
			attribute redefines o = 15;
			attribute redefines p = 16;
			attribute redefines q = 17;
			attribute redefines r = 18;
			attribute redefines s = 19;
		}
	}

	calc def Nested :> Query {
		in root : Element;
		Project(
			source = Tree(source = WhereType(source = Descendants(source = root), type = "PartUsage")),
			properties = ("name", "documentation", "mass")
		)
	}

	calc def Wide :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s")
		)
	}

	part def SizedReport :> Document {
		attribute redefines title = "Sized Report";
		part parts : Table {
			attribute redefines caption = "Parts by owner";
			attribute redefines columnWidths = (774, 206, 0);
			calc rows : Nested { in root = telescope; }
		}
		part readings : Table {
			attribute redefines caption = "Readings";
			calc rows : Wide { in root = sensors; }
		}
	}
}
