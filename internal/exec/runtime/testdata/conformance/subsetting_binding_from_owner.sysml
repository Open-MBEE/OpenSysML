// A binding an owner declares through a path (`child.picked`) constrains the nested subsetting
// feature as one of its own does: `picked` holds the value of `coll` that `chosen` holds
// (KerML 1.0 §7.3.4.4, §7.4.6), then another value of `coll`.
package test {
    item def Node;
    part def Child {
        item a : Node;
        item b : Node;
        item c : Node;
        item coll : Node [3] = (a, b, c);
        item picked : Node [2] :> coll;
    }
    part def Parent {
        part child : Child;
        item chosen : Node [1] = child.c;
        binding [1] bind [0..1] child.picked = [0..1] chosen;
    }
    part p : Parent;
}
