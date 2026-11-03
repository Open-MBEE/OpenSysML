// A subsetting feature stating no value of its own holds values of the feature it
// subsets, as many as its lower bound asks (KerML 1.0 §7.3.4.4); a binding naming one
// of them picks which (§7.4.6). `two` holds two of `coll`; `picked` holds the value
// of `coll` that `late` also holds.
package test {
    item def Node;
    part def Pool {
        item x : Node;
        item y : Node;
        item z : Node;
        item w : Node;
        item coll : Node [4] = (x, y, z, w);
        item late : Node [2] = (z, w);
        item two : Node [2] :> coll;
        item picked : Node [1] :> coll;
        binding [1] bind [0..1] picked = [0..1] late;
    }
    part pool : Pool;
}
