package RoleInheritance {
    private import OpenSysMLRenderings::*;

    part def Person;
    part def Vehicle;

    use case def Base {
        actor driver : Person;
        subject s : Vehicle;
        objective {
            doc /* Arrive safely. */
        }
    }

    use case def Trip :> Base;
    use case def Trip2 :> Base {
        actor :>> driver;
    }

    view trip {
        expose Trip;
        render asCaseDiagram;
    }

    view trip2 {
        expose Trip2;
        render asCaseDiagram;
    }
}
