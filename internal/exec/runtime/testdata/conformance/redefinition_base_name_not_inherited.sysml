// A redefined feature is not inherited under its name by the type owning the
// redefinition, nor by what specializes that type (KerML 1.0 §7.3.4.5,
// §8.3.3.1.10 removeRedefinedFeatures): a chain reading the removed name is
// unresolved on the evaluator as it is for the static read, whether the
// redefinition is written (`own :>> mass`) or a case role's (`subject s`
// redefining `Cases::Case::subj`), while the redefining name and a name an
// unnamed redefinition takes (`objective { … }` as `obj`) read the one value.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::size;

    part def Comp { attribute mass : Real = 0.0; }
    part def Sys :> Comp { attribute own :>> mass = 1.5; }
    part def Bus :> Sys { attribute dry :>> own = 4.0; }
    part bus : Bus { attribute :>> dry = 7.0; }

    use case def U {
        subject s : Integer = 3;
        objective { doc /* o */ }
    }
    use case u : U;
    use case def Named {
        subject s : Integer = 5;
        objective o { doc /* o */ }
    }
    use case named : Named;

    part reader {
        attribute busDry : Real = bus.dry;
        attribute busOwn : Real = bus.own;
        attribute busMass : Real = bus.mass;
        attribute subjectName : Integer = u.s;
        attribute subjectRole : Integer = u.subj;
        attribute subjectCount : Integer = u.s->size();
        attribute objectiveRole : Boolean = u.obj->size() == 1;
        attribute namedSubjectRole : Integer = named.subj;
        attribute namedObjectiveRole : Boolean = named.obj->size() == 1;
        attribute namedObjective : Boolean = named.o->size() == 1;
    }
}
