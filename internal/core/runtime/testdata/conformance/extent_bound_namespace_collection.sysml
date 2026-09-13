// A namespace-level usage of several occurrences given a value is bound to that value for the
// run (KerML 1.0 §7.4.11), so it denotes the objects its value yields rather than a lower bound
// of anonymous ones: `all Wheel` reaches those two and no other, the usage reads as them, and
// a usage bound to another's members denotes those members rather than objects of its own.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel { attribute radius : Real = 1.0; }

    part wheels : Wheel[2] = (new Wheel(), new Wheel());
    ref part front : Wheel[2] = wheels;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Wheel) == 2,
            size(wheels) == 2,
            wheels#(1) === (all Wheel)#(1),
            wheels#(2) === (all Wheel)#(2),
            wheels#(1) === wheels#(1),
            front#(2) === wheels#(2),
            wheels.radius == (1.0, 1.0)
        );
    }
}
