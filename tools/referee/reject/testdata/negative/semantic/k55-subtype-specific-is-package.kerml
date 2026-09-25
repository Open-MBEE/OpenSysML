// Invalid: the specific of a keyword-first specialization must be a type, not a package (KerML 8.3.3.1 Specialization::specific is typed Type; pilot fails to link the Specialization — `Couldn't resolve reference to Type 'Q'` — because that position admits only Types).
package K55SubtypeSpecificIsPackage {
    package Q;
    class B;
    specialization subtype Q :> B;
}
