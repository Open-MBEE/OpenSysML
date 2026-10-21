// A variation point reports `variation` and `abstract` (a variation is always
// abstract, with no keyword written), a variant `variant`; the same model is
// read for the variation usage, the enumeration and an ordinary part.
package PLE {
	part def Engine;
	part def Transmission;
	part fourCyl : Engine;
	part sixCyl : Engine;
	part manual : Transmission;

	variation part def EngineChoices :> Engine {
		variant fourCyl;
		variant sixCyl;
	}

	part family {
		part engine : EngineChoices;
		variation part transmission : Transmission {
			variant manual;
		}
	}

	enum def Color { red; blue; }
}
