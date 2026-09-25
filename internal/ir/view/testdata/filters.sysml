metadata def Safety;

package Systems {
	package Braking {
		#Safety part def Brake {
			part pad;
		}
		part def Pedal;
	}

	#Safety part def Airbag;
	part def Radio;
}

package FilteredViews {
	private import Views::*;

	// The elements a filtered recursive expose reaches, inherited by every view
	// typed by this definition.
	view def SafetyView {
		expose Systems::**[@Safety];
	}

	view safetyView : SafetyView {
		expose Systems::Radio;
	}
}
