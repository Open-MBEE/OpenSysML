// `binding [1] bind [0..1] left = [0..1] right` identifies one value of `left` with one
// value of `right` without saying which (KerML 1.0 §7.4.6, §8.3.4.5.2), so neither end
// is determined by it: each holds what its own declaration states, and validating the
// object checks that the ends share a value: `corner` is a value of both, so the binding holds.
package test {
    item def Node;
    part def Mesh {
        item nodes : Node [4];
        item left : Node [2] :> nodes;
        item right : Node [2] :> nodes;
        item corner : Node [1] :> left, right;
        binding [1] bind [0..1] left = [0..1] right;
    }
    part mesh : Mesh;
}
