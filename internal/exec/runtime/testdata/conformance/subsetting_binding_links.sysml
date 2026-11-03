// A binding stating two links between a subsetting feature and another collection picks
// two of the subsetted feature's values, those the other end holds (KerML 1.0 §7.3.4.4,
// §7.4.6); the rest of the lower bound comes from the subsetted feature.
package test {
    item def Node;
    part def Pool {
        item a : Node;
        item b : Node;
        item c : Node;
        item d : Node;
        item coll : Node [4] = (a, b, c, d);
        item late : Node [2] = (c, d);
        item picked : Node [3] :> coll;
        binding [2] bind [0..2] picked = [0..2] late;
    }
    part pool : Pool;
}
