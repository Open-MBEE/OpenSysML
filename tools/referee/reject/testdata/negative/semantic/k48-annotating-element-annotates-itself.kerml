// Invalid: an annotation owned by its annotated element must own its annotating element, so an annotating element cannot annotate itself (KerML 8.3.2.3.3 validateAnnotationAnnotatedElementOwnership; pilot validateAnnotationAnnotatedElementOwnership).
package K43AnnotatingElementAnnotatesItself {
    class A;
    metaclass M;
    comment C1 about C1 /* a comment annotating itself */
    comment C2 about A, C2 /* and one annotating itself beside another element */
    metadata m3 : M about m3;
}
