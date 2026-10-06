package VehicleUseCases {
	part def Vehicle;
	part def Person;

	use case def 'Add Fuel' {
		subject vehicle : Vehicle;
		actor fueler : Person;
	}
	use case def 'Enter Vehicle' {
		subject vehicle : Vehicle;
		actor driver : Person;
	}
	use case 'Provide Transportation' {
		subject vehicle : Vehicle;
		actor driver : Person;
		actor passengers : Person[0..4];
		objective {
			doc /* Deliver the driver and passengers to their destination. */
		}
		include use case addFuel : 'Add Fuel'[0..*] {
			subject vehicle;
			actor fueler = driver;
		}
		include use case 'enter vehicle' : 'Enter Vehicle' {
			subject vehicle;
			actor driver = 'Provide Transportation'::driver;
		}
	}
}

package UseCaseViews {
	private import StandardViewDefinitions::*;

	view useCaseView : GeneralView {
		filter @SysML::UseCaseUsage;
		expose VehicleUseCases::'Provide Transportation';
	}
	view caseDefinitionView : GeneralView {
		expose VehicleUseCases::*[@SysML::CaseDefinition];
	}
}
