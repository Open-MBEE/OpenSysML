// Invalid: A view definition may have at most one view rendering (SysML 7.25 Views and Viewpoints; pilot validateViewDefinitionOnlyOnvViewRendering).
package S40ViewDefTwoRenderings {
    rendering def RD;
    view def V {
        render rendering r1 : RD;
        render rendering r2 : RD;
    }
}
