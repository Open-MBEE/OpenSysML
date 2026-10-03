package CaseExamples {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import OpenSysMLViews::*;
	private import DiagramLayout::*;

	part def Vehicle;
	part def Person;
	part def Equipment;

	requirement def FuelReady {
		subject vehicle : Vehicle;
	}

	use case def 'Add Fuel' {
		subject vehicle : Vehicle;
		actor fueler : Person;
	}

	use case def 'Enter Vehicle' {
		subject vehicle : Vehicle;
		actor driver : Person;
	}

	use case 'Provide Transportation' : 'Enter Vehicle' {
		subject vehicle : Vehicle;
		actor driver : Person;
		objective {
			doc /* Transport the vehicle and its passengers safely. */
		}

		include 'Add Fuel'[0..*] {
			subject vehicle;
			actor fueler = driver;
		}

		include use case 'enter vehicle' : 'Enter Vehicle' {
			subject vehicle;
			actor driver = 'Provide Transportation'::driver;
		}

		use case 'Drive Vehicle' {
			subject vehicle;
			actor driver = 'Provide Transportation'::driver;
		}
	}

	analysis def FuelAnalysis {
		subject vehicle : Vehicle;
		objective {
			doc /* Estimate the fuel remaining after a journey. */
		}
	}

	verification def FuelVerification {
		subject vehicle : Vehicle;
		objective {
			verify FuelReady;
		}
	}

	part def Tool;

	view caseDiagram {
		expose CaseExamples::'Provide Transportation';
		expose CaseExamples::FuelAnalysis;
		expose CaseExamples::FuelVerification;
		expose CaseExamples::Tool;
		render asCaseDiagram;
	}

	view caseDefinition : CaseView {
		expose CaseExamples::'Provide Transportation';
	}

	view caseLayout {
		expose CaseExamples::'Provide Transportation';
		render asCaseDiagram;
		metadata Layout about CaseExamples::'Provide Transportation' { x = 40; y = 60; }
		metadata Layout about CaseExamples::'Provide Transportation'::driver { x = 100; y = 60; }
		metadata Route about CaseExamples::'Provide Transportation'::'Add Fuel' { points = (60, 20, 100, 60); }
	}
}
