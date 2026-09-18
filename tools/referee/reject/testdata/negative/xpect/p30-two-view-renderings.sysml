// Invalid: A view may have at most one view rendering (SysML 7.23 validateViewUsageOnlyOneRendering; pilot ViewRendering_invalid.sysml.xt:31).
package P30TwoViewRenderings {
    rendering r;
    view v {
        render rendering r4;
        render rendering r5;
    }
}
