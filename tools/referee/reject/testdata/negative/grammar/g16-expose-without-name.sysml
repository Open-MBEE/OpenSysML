// Invalid: NamespaceExpose requires an imported namespace or filter package after the expose prefix (SysML.xtext:2383 NamespaceExpose).
package G16ExposeWithoutName {
    part def Engine;
    view def V {
        expose ;
    }
}
