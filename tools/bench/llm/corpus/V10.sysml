package WindFarm {
    private import StandardViewDefinitions::*;

    part def Person;
    part def Turbine;
    part def Substation;
    part def Cable;

    part def WindFarm {
        doc /* An offshore wind farm feeding the grid through one substation. */
        part turbines : Turbine[1..*];
        part substation : Substation;
        part cables : Cable[1..*];
    }

    requirement def OutputRequirement {
        doc /* The farm shall deliver its rated output at nominal wind speed. */
    }

    requirement def AvailabilityRequirement {
        doc /* The farm shall be available 95 percent of the year. */
    }

    concern def GridStability {
        doc /* Output variations must not destabilise the grid. */
        subject farm : WindFarm;
        stakeholder gridOperator : Person;
    }

    concern def Maintenance {
        doc /* Turbines must be reachable for repair in rough seas. */
        subject farm : WindFarm;
        stakeholder maintenanceCrew : Person;
    }

    viewpoint def OperatorViewpoint {
        frame concern stability : GridStability;
    }

    viewpoint def MaintenanceViewpoint {
        frame concern maintenance : Maintenance;
    }

    rendering def BrowserRendering;
    rendering asBrowser : BrowserRendering;

    view def OperatorView specializes GridView {
        satisfy requirement operatorPerspective : OperatorViewpoint;
    }

    view def MaintenanceView specializes BrowserView {
        satisfy requirement maintenancePerspective : MaintenanceViewpoint;
    }

    view operatorRequirements : OperatorView {
        expose OutputRequirement;
        expose AvailabilityRequirement;
    }

    view farmLayout : MaintenanceView {
        expose WindFarm::**;
        render asBrowser;
    }

    comment about WindFarm /* Sized for forty turbines in the first phase. */
}
