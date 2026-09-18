// Invalid: the general of a keyword-first specialization must be a type, not a package (KerML 8.3.3.1 Specialization::general is typed Type; pilot fails to link the Specialization — `Couldn't resolve reference to Type 'Q'`).
package K56SubtypeGeneralIsPackage {
    package Q;
    class B;
    subtype B specializes Q;
}
