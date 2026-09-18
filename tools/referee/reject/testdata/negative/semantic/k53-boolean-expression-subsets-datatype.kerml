// Invalid: a boolean expression is a feature, so what it subsets must be a feature, not a data type (KerML 8.3.3.3 Subsetting::subsettedFeature is typed Feature and 8.3.4.8 BooleanExpression is an Expression, hence a Step and a Feature; pilot fails to link the Subsetting — `Couldn't resolve reference to Feature 'D'` — because that position admits only Features, exactly as for `feature f :> D`).
package K53BooleanExpressionSubsetsDataType {
    datatype D;
    bool b :> D;
}
