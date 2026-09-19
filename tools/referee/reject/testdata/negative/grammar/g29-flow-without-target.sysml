// Invalid: FlowUsage requires a flow declaration with both ends (SysML.xtext:1269 FlowUsage, :1289 FlowDeclaration).
package G29FlowWithoutTarget {
    part def D {
        attribute a;
        attribute b;
        flow f from a to ;
    }
}
