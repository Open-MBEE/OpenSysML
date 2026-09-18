// Invalid: a metadata body feature must redefine a feature of the metaclass (KerML 8.3.4.11 validateMetadataFeatureBody; pilot validateMetadataFeatureBody).
package K36MetadataBodyRedefinesOutside {
    metaclass M;
    class C;
    feature g : C;
    @M about C {
        :>> g;
    }
}
