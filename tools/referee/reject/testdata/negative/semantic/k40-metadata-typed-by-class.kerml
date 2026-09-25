// Invalid: a metadata feature must be typed by a metaclass, not an ordinary class (KerML 8.3.4.11 validateMetadataFeatureMetadata; pilot validateMetadataFeatureMetadataNotAbstract, because its grammar links `@M` only to a metaclass (KerML.xtext MetadataTyping), so the unlinked feature keeps only the implicit abstract Metaobject type).
package K40MetadataTypedByClass {
    class M;
    class C;
    @M about C;
}
