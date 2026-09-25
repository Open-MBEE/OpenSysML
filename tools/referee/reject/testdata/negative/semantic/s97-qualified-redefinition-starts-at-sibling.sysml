// Invalid: the first segment of a qualified redefinition target is resolved like a simple one, so it cannot name a sibling member of the owning type (KerML 8.2.3.5.2 redefinition name resolution; pilot `Couldn't resolve reference to Feature 'C::nope'`).
package S97QualifiedRedefinitionStartsAtSibling {
    part def C {
        attribute c1;
    }
    part def D :> C {
        part C {
            attribute nope;
        }
        attribute d :>> C::nope;
    }
}
