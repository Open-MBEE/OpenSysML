// Invalid: a variable feature must be owned by an occurrence type (KerML 8.3.3.3 validateFeatureOwnedVariable; pilot validateFeatureIsVariable, Feature_variable_invalid.kerml.xt).
package P03VariableInDatatype {
    datatype D {
        var feature z;
    }
}
