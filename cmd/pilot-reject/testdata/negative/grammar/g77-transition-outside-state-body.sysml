// Invalid: TransitionUsageMember is a StateBodyItem; a part def body admits no 'transition' (SysML.xtext:1754 StateBodyItem, :514 DefinitionBodyItem).
package G77TransitionOutsideStateBody {
    part def D {
        state s1;
        state s2;
        transition first s1 then s2;
    }
}
