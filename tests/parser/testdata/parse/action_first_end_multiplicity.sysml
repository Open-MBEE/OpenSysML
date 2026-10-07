// A `first` end carrying a crossing multiplicity is a succession-as-usage
// (SysML.xtext:998 ConnectorEnd, OwnedCrossMultiplicityMember), not an initial
// node whose member element is a plain name (SysML.xtext:1384 InitialNodeMember).
action def FirstEndMultiplicity {
    action a1;
    action a2;
    action a3;
    first [0..1] a1 then [0..1] a2;
    first [1] a2 then a3;
    first a1 then [0..1] a3;
    first a3 then [1] done {
        doc /* a target end with a body */
    }
    // A plain two-ended `first` keeps its InitialNodeMember reading, which the
    // lowering treats as the succession start -> a1 as before.
    first start then a1;
}
