package InheritedNameCaseRolesClean {
    part def Vehicle;
    part def Person;
    requirement def R {
        subject v : Vehicle;
        actor driver : Person;
        stakeholder owner : Person;
    }
    // Roles under the inherited names redefine the inherited roles.
    requirement def SameNames :> R {
        subject v : Vehicle;
        actor driver : Person;
        stakeholder owner : Person;
    }
    // Renamed roles redefine by position, so the old names are free again.
    requirement def Renamed :> R {
        subject w : Vehicle;
        actor pilot : Person;
        stakeholder payer : Person;
        ref v : Vehicle;
        part driver : Person;
        part owner : Person;
    }
    requirement r : R {
        subject v : Vehicle;
        actor driver : Person;
    }
    use case def U {
        subject s : Vehicle;
        actor a : Person;
        objective o { doc /* base */ }
    }
    use case def U1 :> U {
        subject s : Vehicle;
        actor a : Person;
        objective o { doc /* refined */ }
    }
    use case u2 : U {
        subject s : Vehicle;
        actor b : Person;
        part a : Person;
    }
    concern def C { subject c : Vehicle; stakeholder s1 : Person; }
    concern def C1 :> C { subject c : Vehicle; stakeholder s1 : Person; }
    analysis def A { subject s : Vehicle; objective obj {} return r : ScalarValues::Real; }
    analysis def A1 :> A { subject s : Vehicle; objective obj {} return r : ScalarValues::Real; }
    verification def V { subject s : Vehicle; }
    verification def V1 :> V { subject t : Vehicle; ref s : Vehicle; }
    // A subject redefines the library's, whose name an ordinary member may then take.
    use case def LibraryName { subject s : Vehicle; ref subj : Vehicle; }
    requirement def LibraryName2 { subject s : Vehicle; ref subj : Vehicle; }
}
