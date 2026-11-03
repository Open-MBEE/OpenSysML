// `typed by` is the KerML spelling of a typing (KerML.xtext:600 TypedBy); SysML.xtext:395
// DefinedByKeyword spells it `:` or `defined by`. The parser reads all three alike and
// records the KerML spelling for the notation pass to report in a .sysml file.
package TypedByUsage {
    part def P;
    attribute def A;
    metadata def Tag;

    part p typed by P;
    part q : P;
    part r defined by P;
    attribute a typed by A, A;
    part s typed by P, P;
    part def D {
        @ m typed by Tag;
        @ n defined by Tag;
        @ o : Tag;
    }
}
