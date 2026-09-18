// Invalid: a portion feature cannot be variable (KerML 8.3.3.3 validateFeaturePortionNotVariable; pilot validateFeaturePortionNotVariable).
package K10PortionVarFeature {
    class C specializes Occurrences::Occurrence {
        portion var feature p : C;
    }
}
