// KerML.xtext does not declare SysML's literals, so each names a feature here.
// The Kernel Semantic Library relies on this (`in frame : SpatialFrame[1]`).
package W7CContextualNamesKerML {
    class T;

    feature frame : T;
    feature render : T;
    feature rendering : T;
    feature entry : T;
    feature exit : T;
    feature accept : T;
    feature state : T;
    feature part : T;
    feature view : T;
    feature viewpoint : T;
    feature action : T;
    feature requirement : T;
    feature concern : T;
    feature satisfy : T;
    feature send : T;
    feature item : T;
    feature port : T;
    feature attribute : T;
    feature calc : T;
    feature use : T;
    feature variant : T;
    feature until : T;
    feature when : T;
    feature while : T;
    feature via : T;
    feature at : T;
    feature after : T;

    behavior B {
        step s;
        feature chained : T subsets frame;
    }
}
