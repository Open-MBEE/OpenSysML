// Invalid: the disjoining type of a keyword-first disjoining must be a type, not a package (KerML 8.3.3.3 Disjoining::disjoiningType is typed Type; pilot fails to link the Disjoining — `Couldn't resolve reference to Type 'Q'`).
package K71DisjoiningTypeIsPackage {
    package Q;
    class B;
    disjoining disjoint B from Q;
}
