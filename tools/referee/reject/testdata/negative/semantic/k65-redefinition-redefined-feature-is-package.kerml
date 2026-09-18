// Invalid: the redefined feature of a keyword-first redefinition must be a feature, not a package (KerML 8.3.4.6 Redefinition::redefinedFeature is typed Feature; pilot fails to link the Redefinition — `Couldn't resolve reference to Feature 'Q'`).
package K65RedefinedFeatureIsPackage {
    package Q;
    class A { feature f; }
    specialization redefinition A::f redefines Q;
}
