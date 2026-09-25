// Invalid: ViewRenderingMember requires a rendering usage after the render keyword (SysML.xtext:2335 ViewRenderingMember).
package G17RenderWithoutUsage {
    view def V {
        render ;
    }
}
