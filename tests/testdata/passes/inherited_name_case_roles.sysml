package InheritedNameCaseRoles {
    part def Vehicle;
    part def Person;
    requirement def R {
        subject v : Vehicle;
        actor driver : Person;
        stakeholder owner : Person;
        attribute limit : ScalarValues::Real;
        part helper : Person;
    }
    // Ordinary members under an inherited name.
    requirement def Ordinary :> R {
        attribute limit : ScalarValues::Real;
        part helper : Person;
    }
    // A part under the name of an inherited actor or stakeholder it does not redefine.
    requirement def RoleNames :> R {
        part driver : Person;
        part owner : Person;
    }
    // A second actor has no counterpart at its position, so it redefines nothing.
    requirement def SecondActor :> R {
        subject v : Vehicle;
        actor a1 : Person;
        actor helper : Person;
    }
    use case def U {
        subject s : Vehicle;
        actor a : Person;
        objective o { doc /* base */ }
        part extra : Person;
    }
    use case def U1 :> U {
        subject s : Vehicle;
        actor a : Person;
        objective o { doc /* refined */ }
        part extra : Person;
    }
    use case u2 : U {
        subject s : Vehicle;
        part extra : Person;
    }
    requirement r6 : R {
        attribute limit : ScalarValues::Real;
    }
    concern def C { subject c : Vehicle; stakeholder s1 : Person; part x : Person; }
    concern def C1 :> C { subject c : Vehicle; stakeholder s1 : Person; part x : Person; }
    analysis def A { subject s : Vehicle; objective obj {} return r : ScalarValues::Real; part q : Person; }
    analysis def A1 :> A { subject s : Vehicle; objective obj {} return r : ScalarValues::Real; part q : Person; }
    verification def V { subject s : Vehicle; part q : Person; }
    verification def V1 :> V { subject t : Vehicle; part q : Person; }
    // No subject of its own: the library's stays inherited under its name.
    requirement def NoSubject { ref subj : Vehicle; }
}
