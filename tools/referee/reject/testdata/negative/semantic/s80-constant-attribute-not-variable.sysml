// Invalid: only a variable feature can be constant, and an attribute of a data type is not variable (KerML 8.3.3.3 validateFeatureConstantIsVariable; pilot validateFeatureConstantIsVariable).
package S80ConstantAttributeNotVariable {
    attribute def A;
    attribute def B {
        constant attribute x : A;
    }
}
