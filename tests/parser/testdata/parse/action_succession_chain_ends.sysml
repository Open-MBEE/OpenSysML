// A succession end is a connector end (SysML.xtext ConnectorEnd → OwnedFeatureChain), so every
// notation stating one in an action body takes a feature chain: `first`, `then`, `if`, `else`.
package ChainEnds {
	part def Vehicle {
		action close;
		action lock;
		action open;
	}

	action def Drive {
		part v : Vehicle;
		in go : ScalarValues::Boolean;

		first start then v.close;
		succession closing first v.close then v.lock;
		action idle;
		then v.open;
		then [1] v.close;
		then decide;
			if go then v.lock;
			else v.open;
	}
}
