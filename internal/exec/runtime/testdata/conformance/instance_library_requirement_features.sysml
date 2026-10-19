// A requirement def inherits RequirementCheck's subject, actors, stakeholders and
// constraints; its result parameter answers the object's own check, undetermined
// while the subject is unbound, and `requirement :>> self` stays in the frame.
package test {
    private import ScalarValues::*;

    part def Vehicle {
        attribute mass : Real = 1200.0;
    }

    requirement def MassLimit {
        subject v : Vehicle;
        attribute limit : Real = 1500.0;
        require constraint { v.mass <= limit }
    }
}
