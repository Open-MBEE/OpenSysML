// A variant carries the usage prefix of the element it declares
// (SysML.xtext VariantUsageElement): modifiers, prefix metadata, a portion kind.
package P {
    port def PD;
    metadata def M;
    port def Q {
        port p1 : PD;
        variation port vp : PD {
            variant ref port a : PD;
            variant in port b : PD;
            variant end port c : PD;
            variant abstract port d : PD;
            variant derived ref #M port e : PD;
            variant snapshot port f : PD;
            variant timeslice g;
            variant p1;
        }
    }
}
