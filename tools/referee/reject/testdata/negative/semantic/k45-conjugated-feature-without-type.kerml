// Invalid: a feature must have at least one type, and a feature conjugating a class takes no type from it and owns no implicit subsetting to supply one (KerML 8.3.3.3.4 validateFeatureHasType; pilot validateFeatureHasType).
package K45ConjugatedFeatureWithoutType {
    class D;
    feature f ~ D;
    class C { feature g ~ D; }
}
