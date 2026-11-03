package RoleInheritance {
    private import OpenSysMLRenderings::*;

    part def Person;
    part def Vehicle;

    use case def Base {
        subject s : Vehicle;
        actor driver : Person;
        objective {
            doc /* Arrive safely. */
        }
    }

    use case def Trip :> Base;
    use case def Trip2 :> Base {
        subject :>> s;
        actor :>> driver;
    }
    use case def X;

    view trip {
        expose Trip;
        render asCaseDiagram;
    }

    view trip2 {
        expose Trip2;
        render asCaseDiagram;
    }

    view x {
        expose X;
        render asCaseDiagram;
    }
}
