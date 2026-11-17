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
	// that specializes this one.
	view safetyElements {
		expose Systems::**[@Safety];
	}

	view def SafetyView;

	view safetyView : SafetyView :> safetyElements {
		expose Systems::Radio;
	}
}
