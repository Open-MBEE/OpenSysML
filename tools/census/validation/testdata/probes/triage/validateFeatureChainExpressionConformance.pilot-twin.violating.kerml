// Census: validateFeatureChainExpressionConformance
// Case: violating
// Ours: e[feature-reference-referent] Must be a valid feature
// Pilot: e: Must be a valid feature
package K41ChainExpressionToNestedClass {
    class A {
        class N;
    }
    feature a : A;
    feature f : A = a.N;
}
