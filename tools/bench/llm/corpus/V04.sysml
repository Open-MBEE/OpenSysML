package Hospital {
    part def Person;
    part def Hospital;

    concern def PatientSafety {
        doc /* Patients must never be harmed by equipment failure. */
        subject hospital : Hospital;
        stakeholder clinician : Person;
    }

    viewpoint def SafetyViewpoint {
        frame concern safety : PatientSafety;
    }

    view def SafetyView {
        satisfy requirement safetyPerspective : SafetyViewpoint;
    }
}
