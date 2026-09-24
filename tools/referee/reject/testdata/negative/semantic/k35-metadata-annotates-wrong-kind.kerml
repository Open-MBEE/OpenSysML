// Invalid: a metadata feature's annotated element must conform to its annotatedElement redefinition (KerML 8.3.4.11 validateMetadataFeatureAnnotatedElement; pilot validateMetadataFeatureAnnotatedElement).
package K35MetadataAnnotatesWrongKind {
    metaclass M {
        :>> annotatedElement : KerML::Class;
    }
    class C {
        feature f : C;
    }
    @M about C::f;
}
