// Invalid: a return parameter membership must be owned by a function or expression (KerML 8.3.4.7 validateReturnParameterMembershipOwningType; pilot validateReturnParameterMembershipOwningType).
package K44ReturnParameterInClassifier {
    datatype Count;
    classifier Counter {
        return total : Count;
    }
}
