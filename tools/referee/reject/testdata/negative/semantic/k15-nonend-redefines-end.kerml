// Invalid: a feature redefining an end feature within an association must be an end (KerML 8.3.3.3 validateRedefinitionEndConformance; pilot validateRedefinitionEndConformance).
package K15NonendRedefinesEnd {
    class T;
    assoc A {
        end a : T;
        end b : T;
    }
    abstract assoc B specializes A {
        feature c : T redefines a;
        end d : T redefines b;
    }
}
