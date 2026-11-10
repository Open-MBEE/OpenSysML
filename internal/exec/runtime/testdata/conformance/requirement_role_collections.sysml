// A requirement's actors and stakeholders fill Requirements::RequirementCheck::actors and
// stakeholders, its framed concern concerns (SysML v2 §8.3.11.3, §8.3.21.4), referential or not.
package test {
    private import ScalarValues::*;

    part def Person { attribute id : Integer; }
    concern def Safety;

    requirement def R {
        subject s : Person;
        actor driver : Person;
        stakeholder owner : Person;
        frame concern sf : Safety;
    }
    requirement r : R {
        subject :>> s;
        actor :>> driver;
        stakeholder :>> owner;
        actor passenger : Person;
        stakeholder regulator : Person;
    }
}
