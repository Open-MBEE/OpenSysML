// Invalid: FramedConcernMember is a RequirementBodyItem; a view body admits no 'frame' (SysML.xtext:2062 RequirementBodyItem, :2359 ViewBodyItem).
package G82FrameInViewBody {
    concern def C;
    view V {
        frame concern c : C;
    }
}
