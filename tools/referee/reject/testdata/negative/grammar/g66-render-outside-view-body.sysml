// Invalid: ViewRenderingMember is a ViewBodyItem; a part def body admits no 'render' (SysML.xtext:2335 ViewRenderingMember, :2359 ViewBodyItem; pilot validateViewRenderingMembershipOwningType).
package G66RenderOutsideViewBody {
    rendering def Table;
    part def System {
        render rendering r : Table;
    }
}
