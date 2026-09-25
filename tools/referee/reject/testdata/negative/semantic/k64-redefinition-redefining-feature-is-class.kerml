// Invalid: the redefining feature of a keyword-first redefinition must be a feature, not a class (KerML 8.3.4.6 Redefinition::redefiningFeature is typed Feature; pilot fails to link the Redefinition — `Couldn't resolve reference to Feature 'C'`).
package K64RedefiningFeatureIsClass {
    class C;
    class A { feature f; }
    redefinition C :>> A::f;
}
