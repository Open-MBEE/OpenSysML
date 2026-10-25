// The split of binding_partial_collections: its ends share no value, so both bindings
// are violated — the second one necessarily, an item never being an integer.
package test {
    private import ScalarValues::*;
    item def Node;
    part def Split {
        item nodes : Node [4];
        item left : Node [2] :> nodes;
        item right : Node [2] :> nodes;
        attribute counts : Integer [2] = (1, 2);
        binding [1] bind [0..1] left = [0..1] right;
        binding [1] bind [0..1] left = [0..1] counts;
    }
    part split : Split;
}
