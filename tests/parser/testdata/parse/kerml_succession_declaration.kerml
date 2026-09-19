// A succession may state a whole feature declaration — a name or a
// specialization, a type and a multiplicity — before its ends
// (KerML.xtext:891 SuccessionDeclaration, first alternative).
package SuccessionDeclaration {
	behavior Paint;
	behavior Dry;

	assoc Paint_Before_Dry {
		end feature paint : Paint;
		end feature dry : Dry;
	}

	behavior Base {
		step paint : Paint;
		step dry : Dry;

		succession named : Paint_Before_Dry [1] first paint then dry;
	}

	// A redefinition names an inherited feature, so the succession it redefines
	// is declared in the general behavior, not alongside it.
	behavior Process specializes Base {
		succession redefines named : Paint_Before_Dry [1] first paint then dry;
		succession first paint then dry;
		succession [1] first paint then dry;
	}
}
