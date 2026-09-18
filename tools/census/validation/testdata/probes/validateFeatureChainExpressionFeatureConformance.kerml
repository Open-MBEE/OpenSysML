// Census: validateFeatureChainExpressionFeatureConformance
// Expect: error: Must be a valid feature
package K41ChainExpressionToNestedClass {
    class A {
        class N;
    }
    feature a : A;
    feature f : A = a.N;
}
