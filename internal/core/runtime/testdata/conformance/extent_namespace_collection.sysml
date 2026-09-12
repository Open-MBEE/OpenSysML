// A namespace-level usage of several occurrences and no value denotes its lower bound of
// objects for the run (KerML 1.0 §7.3.4.3: a feature of multiplicity [2] has exactly two values),
// created once and in declaration order, as a usage of one occurrence does: `all Wheel` counts
// them, the usage reads as the sequence of them, and a chain through it reads each of them. A
// `[1..*]` usage denotes one object, a `[0..*]` usage none; a read of either is undetermined of
// its count, as a nested collection of open count reads, so only the extents are probed here.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel { attribute radius : Real = 1.0; }
    item def Link;
    part def Hub;

    part wheels : Wheel[2];
    item links : Link[3];
    part hubs : Hub[1..*];
    part spares : Wheel[0..*];
    part spare : Wheel;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Wheel) == 3,
            size(all Link) == 3,
            size(all Hub) == 1,
            size(wheels) == 2,
            size(links) == 3,
            size(spares) == 0,
            wheels#(1) === (all Wheel)#(1),
            wheels#(2) === (all Wheel)#(2),
            spare === (all Wheel)#(3),
            wheels#(1) === wheels#(1),
            wheels#(1) !== wheels#(2),
            includes(all Link, links#(3)),
            wheels.radius == (1.0, 1.0),
            wheels#(2).radius == 1.0
        );
    }
}
