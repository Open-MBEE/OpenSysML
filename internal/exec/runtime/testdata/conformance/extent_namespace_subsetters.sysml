// The objects a namespace-level collection usage denotes include the objects the usages
// subsetting it denote, anonymous members making up only what the lower bound still asks
// for (KerML 1.0 §7.3.4.4): an abstract usage holds its concrete specializations' values
// alone, and a collection of one is the object its subsetter denotes.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Car;

    part vs : Car[2];
    part c1 : Car[1] :> vs;

    abstract part avs : Car[2];
    part c2 : Car[1] :> avs;
    part c3 : Car[1] :> avs;

    part v : Car[1];
    part c : Car[1] :> v;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Car) == 5,
            size(vs) == 2,
            vs#(1) === c1,
            size(avs) == 2,
            avs#(1) === c2,
            avs#(2) === c3,
            v === c
        );
    }
}
