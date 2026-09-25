// Invalid: the disjoined type of a keyword-first disjoining must be a type, not a package (KerML 8.3.3.3 Disjoining::typeDisjoined is typed Type; pilot fails to link the Disjoining — `Couldn't resolve reference to Type 'Q'`).
package K70TypeDisjoinedIsPackage {
    package Q;
    class B;
    disjoint Q from B;
}
