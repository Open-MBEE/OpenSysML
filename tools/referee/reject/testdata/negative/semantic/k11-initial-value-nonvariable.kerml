// Invalid: only a variable feature can have an initial value (KerML 8.4.4.9 validateFeatureValueIsInitial; pilot validateFeatureValueIsInitial).
package K11InitialValueNonvariable {
    class C {
        feature x : C := null;
    }
}
