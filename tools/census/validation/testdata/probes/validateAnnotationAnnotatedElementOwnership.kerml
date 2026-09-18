// Census: validateAnnotationAnnotatedElementOwnership
// Expect: error: Must own its annotating element
package P {
  class A;
  comment C about A, C /* an annotating element cannot annotate itself */
}
