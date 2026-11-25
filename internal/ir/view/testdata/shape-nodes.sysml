package ShapeExamples {
	private import OpenSysMLRenderings::*;

	part def Person;
	part def Vehicle;

	case def Inspection {
		subject vehicle : Vehicle;
		actor driver : Person;
	}

	requirement def Ready {
		subject vehicle : Vehicle;
	}

	view shapes {
		expose ShapeExamples;
		render asMixedDiagram;
	}
}
