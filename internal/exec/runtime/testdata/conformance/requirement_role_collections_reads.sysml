// The role collections read through a requirement nested in a part, a case's actors
// through Cases::Case::actors, and an actorless requirement's actors as empty.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::size;
    private import SequenceFunctions::includes;
    private import VerificationCases::VerdictKind;

    part def Person { attribute id : Integer; }
    part op : Person { attribute :>> id = 9; }
    concern def Safety;

    requirement def R {
        subject s : Person;
        actor driver : Person { attribute :>> id default 1; }
        stakeholder owner : Person { attribute :>> id default 2; }
        frame concern sf : Safety;
    }
    requirement def Bare { subject s : Person; }
    verification def VD { subject s : Person; actor tester : Person; return verdict : VerdictKind = VerdictKind::pass; }

    part sys {
        requirement r : R {
            subject :>> s;
            actor :>> driver { attribute :>> id = 3; }
            stakeholder :>> owner { attribute :>> id = 4; }
            actor passenger : Person { attribute :>> id = 5; }
        }
        requirement plain : R;
        requirement bare : Bare;
        verification v : VD { subject :>> s = op; actor :>> tester = op; }
    }

    calc def Probe {
        out actorCount : Integer = size(sys.r.actors);
        out stakeholderCount : Integer = size(sys.r.stakeholders);
        out plainActorCount : Integer = size(sys.plain.actors);
        out concernCount : Integer = size(sys.r.concerns);
        out subrequirementCount : Integer = size(sys.r.subrequirements);
        out bareActorCount : Integer = size(sys.bare.actors);
        out driverIsActor : Boolean = includes(sys.r.actors, sys.r.driver);
        out passengerIsActor : Boolean = includes(sys.r.actors, sys.r.passenger);
        out ownerIsStakeholder : Boolean = includes(sys.r.stakeholders, sys.r.owner);
        out caseActorIsOp : Boolean = includes(sys.v.actors, op);
        out actorIds : Integer[0..*] = (sys.r.actors as Person).id;
        out plainActorIds : Integer[0..*] = (sys.plain.actors as Person).id;
        return : Integer = size(sys.r.actors);
    }
    calc probe : Probe;
}
