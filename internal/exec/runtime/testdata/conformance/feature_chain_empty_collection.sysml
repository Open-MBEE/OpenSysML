// A chain over a collection holding nothing has no values, so the roll-up over
// it is the sum's identity rather than an error: RealFunctions::sum's is 0.0.
package test {
    private import ScalarValues::Real;
    private import RealFunctions::*;
    private import SequenceFunctions::*;

    part def Sub {
        attribute volume : Real = 1.0;
    }

    part def Ctx {
        part subsystem : Sub[*];
        attribute totalVolume : Real = sum(subsystem.volume);
        attribute count : Real = size(subsystem.volume);
    }
}
