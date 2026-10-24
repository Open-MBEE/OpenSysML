// Invalid: a feature redefining an end feature within a connector must be an end (KerML 8.3.3.3 validateRedefinitionEndConformance; pilot validateRedefinitionEndConformance).
package K75NonendRedefinesEndInConnector {
    class T;
    assoc A {
        end a : T;
        end b : T;
    }
    struct Ctx {
        feature x : T;
        feature y : T;
        connector c : A from x to y {
            feature d : T redefines a;
        }
    }
}
