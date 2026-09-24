// Prefix metadata annotates a dependency like any other element
// (SysML.xtext:55-57), and a metadata usage names its annotated elements after
// `about` (SysML.xtext:142-149).
package PrefixMetadataMembers {
    metadata def Rationale;
    metadata def Meta;
    part a;
    part b;
    #Rationale dependency d from a to b;
    @Meta about a, b;
    dependency plain from a to b;
}
