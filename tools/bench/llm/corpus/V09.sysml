package Spacecraft {
    private import StandardViewDefinitions::*;

    part def Person;
    part def Bus;
    part def Payload;
    part def Spacecraft {
        part bus : Bus;
        part payload : Payload;
    }

    concern def MassBudget {
        doc /* The launch vehicle limits total mass. */
        subject craft : Spacecraft;
        stakeholder systemsEngineer : Person;
    }

    concern def ScienceReturn {
        doc /* The payload must deliver the mission's science goals. */
        subject craft : Spacecraft;
        stakeholder scientist : Person;
    }

    viewpoint def EngineeringViewpoint {
        frame concern mass : MassBudget;
    }

    viewpoint def ScienceViewpoint {
        frame concern science : ScienceReturn;
    }

    view def EngineeringView specializes BrowserView {
        satisfy requirement engineeringPerspective : EngineeringViewpoint;
    }

    view def ScienceView {
        satisfy requirement sciencePerspective : ScienceViewpoint;
    }

    view busStructure : EngineeringView {
        expose Spacecraft::**;
    }

    view payloadOverview : ScienceView {
        expose Payload;
    }
}
