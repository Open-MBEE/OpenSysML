// A feature holding values rather than objects that states no value of its own holds values
// of the feature it subsets, as many as its lower bound asks (KerML 1.1 §7.3.4.4); a binding
// naming one picks which (§7.4.6). `r` holds `q`'s 4, `top` the first of `faces`, `pair` two
// of `sizes` and `picked` the value of `codes` that `late` also holds.
package test {
    private import ScalarValues::*;
    part def Values {
        attribute q = 4;
        attribute r [1] :> q;
        attribute faces [6] = (1..6);
        attribute top [1] :> faces;
        attribute sizes : Integer [3] = (10, 20, 30);
        attribute pair : Integer [2] :> sizes;
        attribute codes : Integer [3] = (7, 8, 9);
        attribute late : Integer [2] = (8, 9);
        attribute picked : Integer [1] :> codes;
        binding [1] bind [0..1] picked = [0..1] late;
    }
    part vals : Values;
}
