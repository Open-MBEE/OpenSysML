package MixedMemberships {
	private import OpenSysMLRenderings::*;
	private import DiagramLayout::*;

	package Library {
		attribute def Pressure;
		port def FluidPort;
	}

	part def Impeller;

	part def Pump {
		attribute pressure : Library::Pressure;
		port outlet : Library::FluidPort;
		part impeller : Impeller;
		ref part spare : Impeller;
		action prime;
		state armed;
		part def Seal;
	}

	concern def Containment;

	requirement def Safety {
		require constraint minPressure { true }
		assume constraint primedPump { true }
		assert constraint sealed { true }
		frame concern contained : Containment;
	}

	view memberships {
		expose MixedMemberships;
		render asMixedDiagram;
	}

	view membershipsLayout {
		expose MixedMemberships;
		render asMixedDiagram;
		metadata Layout about MixedMemberships::Pump { x = 40; y = 20; }
		metadata Route about MixedMemberships::Pump::impeller { points = (60, 40, 90, 70); }
	}
}
