// Invalid: A feature chain requires a feature after the dot (KerMLExpressions.xtext:448 OwnedFeatureChaining).
package G55FeatureChainTrailingDot {
    part def A { attribute b; }
    part a : A;
    attribute y = a.;
}
