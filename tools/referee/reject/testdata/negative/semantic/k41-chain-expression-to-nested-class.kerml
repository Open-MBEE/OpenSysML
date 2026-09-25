// Invalid: the target of a feature chain expression must be a feature of the source's type (KerML 8.3.4.8 validateFeatureChainExpressionFeatureConformance; pilot validateFeatureChainExpressionFeatureConformance).
package K41ChainExpressionToNestedClass {
    class A {
        class N;
    }
    feature a : A;
    feature f : A = a.N;
}
