// Census: validateMetadataFeatureAnnotatedElement
// Expect: error: Cannot annotate
package P {
  metaclass M specializes Metaobjects::Metaobject { feature redefines annotatedElement : KerML::Feature; }
  #M class C;
}
