package Cases {
	part Operator;
	use case 'Control Instrument' {
		include 'Manipulate Stage';
	}
	use case 'Manipulate Stage';
	connection 'Control Instrument to Operator' connect 'Control Instrument' to Operator;
	connection 'Control Instrument includes Manipulate Stage' connect 'Control Instrument' to 'Manipulate Stage';
}

package CaseViews {
	private import Views::*;

	view useCases {
		expose Cases::Operator;
		expose Cases::'Control Instrument';
		expose Cases::'Manipulate Stage';
		expose Cases::'Control Instrument to Operator';
		expose Cases::'Control Instrument includes Manipulate Stage';
		render asInterconnectionDiagram;
	}
}
