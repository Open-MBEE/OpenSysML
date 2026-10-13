package LabUseCases {
	private import ScalarValues::Boolean;

	part Operator;
	part Administrator;

	use case 'Configure Instrument' {
		subject;
		actor operator :> Operator;
		actor administrator :> Administrator;
	}

	connection 'Operator to Configure Instrument'
		connect Operator to 'Configure Instrument'.operator;
	connection 'Administrator to Configure Instrument'
		connect Administrator to 'Configure Instrument'.administrator;

	// A rendering drawing the actors as parameter symbols on the use case's
	// boundary, with the connections ending at them.
	rendering withParameterSymbols :> Views::asInterconnectionDiagram {
		attribute showParameterSymbols : Boolean default = true;
	}

	// A rendering of the one above redefining the switch off again, and a
	// rendering of it leaving the switch alone: the specialized rendering decides.
	rendering withoutParameterSymbols :> withParameterSymbols {
		attribute :>> showParameterSymbols = false;
	}
	rendering inheritingParameterSymbols :> withParameterSymbols;

	view useCases {
		expose Operator;
		expose Administrator;
		expose 'Configure Instrument';
		expose 'Operator to Configure Instrument';
		expose 'Administrator to Configure Instrument';
		render Views::asInterconnectionDiagram;
	}
	view useCasesWithParameters {
		expose Operator;
		expose Administrator;
		expose 'Configure Instrument';
		expose 'Operator to Configure Instrument';
		expose 'Administrator to Configure Instrument';
		render withParameterSymbols;
	}
	view useCasesWithoutParameters {
		expose Operator;
		expose Administrator;
		expose 'Configure Instrument';
		expose 'Operator to Configure Instrument';
		expose 'Administrator to Configure Instrument';
		render withoutParameterSymbols;
	}
	view useCasesInheritingParameters {
		expose Operator;
		expose Administrator;
		expose 'Configure Instrument';
		expose 'Operator to Configure Instrument';
		expose 'Administrator to Configure Instrument';
		render inheritingParameterSymbols;
	}
}
