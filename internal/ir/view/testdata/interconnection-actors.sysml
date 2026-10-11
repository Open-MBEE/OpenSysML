package Cases {
	part Operator;
	use case 'Control Instrument' {
		subject;
		actor operator :> Operator;
	}
	connection 'Operator to Control Instrument' connect Operator to 'Control Instrument'.operator;
}

package CaseViews {
	private import Views::*;

	view useCases {
		expose Cases::Operator;
		expose Cases::'Control Instrument';
		expose Cases::'Operator to Control Instrument';
		render asInterconnectionDiagram;
	}
}
