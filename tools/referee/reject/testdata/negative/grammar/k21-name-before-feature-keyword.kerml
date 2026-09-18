// Invalid: a feature's name follows its kind keyword; a name written ahead of the keyword begins no body element (KerML.xtext:538 Feature, :548 FeatureDeclaration, :364 TypeBody).
package K21NameBeforeFeatureKeyword {
    class A;
    class B {
        foo feature bar : A;
    }
}
