// Invalid: the leading feature of a chained redefinition target is resolved like a simple one, so it cannot name a sibling member of the owning type (KerML 8.2.3.5.2 redefinition name resolution; pilot `Couldn't resolve reference to Feature 'w'`).
package S98ChainedRedefinitionStartsAtSibling {
    private import ScalarValues::Real;
    part def W {
        attribute x : Real;
    }
    part def B {
        part w : W;
        attribute z :>> w.x;
    }
}
