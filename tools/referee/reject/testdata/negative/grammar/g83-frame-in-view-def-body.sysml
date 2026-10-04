// Invalid: FramedConcernMember is a RequirementBodyItem; a view def body admits no 'frame' (SysML.xtext:2062 RequirementBodyItem, :2329 ViewDefinitionBodyItem).
package G83FrameInViewDefBody {
    concern def C;
    view def V {
        frame c;
    }
}
