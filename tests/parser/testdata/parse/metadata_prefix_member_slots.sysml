// Grammar positions: SysML.xtext LibraryPackage, ActionBodyItem,
// ReturnParameterMember, RequirementVerificationUsage, FramedConcernUsage and ViewRenderingUsage.
metadata def M;

library #M package LibraryPackage;
standard library #M package StandardLibrary;

action def Chain {
    action first;
    then #M action second;
}

calc def Result {
    return #M attribute result : Real;
}

requirement def Requirement {
    frame #M concern concern;
}

verification def Verification {
    verify #M requirement requirement;
}

view def View {
    render #M rendering rendering;
}
