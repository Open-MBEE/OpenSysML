// The roll-up pattern: nested parts subset an unbounded feature, and a chain
// through that feature collects each object's attribute for the total.
package test {
    private import ScalarValues::Real;
    private import RealFunctions::*;

    part def Sub {
        attribute volume : Real;
    }

    part def Ctx {
        part subsystem : Sub[0..*];
        attribute volumes : Real[0..*] = subsystem.volume;
        attribute totalVolume : Real = sum(subsystem.volume);
    }

    part ctx : Ctx {
        part a : Sub :> subsystem {
            attribute :>> volume = 2.0;
        }
        part b : Sub :> subsystem {
            attribute :>> volume = 3.5;
        }
    }
}
