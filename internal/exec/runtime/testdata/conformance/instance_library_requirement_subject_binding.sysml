// A requirement's `subject … = expr` binds the subject, and RequirementCheck's
// inherited `subj` reads the same object through the implicit redefinition.
package test {
    private import ScalarValues::*;

    part def Vehicle {
        attribute mass : Real = 1200.0;
    }
    part car : Vehicle;

    requirement def MassLimit {
        subject v : Vehicle;
        attribute limit : Real = 1500.0;
        require constraint { v.mass <= limit }
    }

    requirement carMassLimit : MassLimit {
        subject v : Vehicle = car;
    }
}
