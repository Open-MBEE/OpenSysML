package ThenPastConnectors {
	action def Step;
	part def Widget;
	port def Plug;

	// A positional `then` sequences from the nearest member before it that is
	// not a connector, so each of these is read past to the member before it.
	action def AfterConnect {
		action a : Step;
		part p : Widget;
		part q : Widget;
		connect p to q;
		then action b : Step;
	}

	action def AfterNamedConnection {
		action a : Step;
		part p : Widget;
		part q : Widget;
		connection c connect p to q;
		then action b : Step;
	}

	action def AfterInterface {
		action a : Step;
		port p1 : Plug;
		port p2 : ~Plug;
		interface i connect p1 to p2;
		then action b : Step;
	}

	action def AfterAllocate {
		action a : Step;
		part p : Widget;
		part q : Widget;
		allocate p to q;
		then action b : Step;
	}

	action def AfterNamedAllocation {
		action a : Step;
		part p : Widget;
		part q : Widget;
		allocation al allocate p to q;
		then action b : Step;
	}

	// Several connectors in a row are all read past.
	action def AfterSeveral {
		action a : Step;
		part p : Widget;
		part q : Widget;
		connect p to q;
		allocate p to q;
		bind p = q;
		then action b : Step;
	}

	// The same rule holds in a structural body.
	part def Structural {
		part a : Widget;
		part p : Widget;
		part q : Widget;
		connect p to q;
		allocate p to q;
		then part b : Widget;
	}

	part def Ports {
		part a : Widget;
		port p1 : Plug;
		port p2 : ~Plug;
		interface i connect p1 to p2;
		then part b : Widget;
	}

	// A connector with a body is read past like one without.
	action def AfterConnectionBody {
		action a : Step;
		part p : Widget;
		part q : Widget;
		connection c connect p to q {
			doc /* p and q are joined */
		}
		then action b : Step;
	}
}
