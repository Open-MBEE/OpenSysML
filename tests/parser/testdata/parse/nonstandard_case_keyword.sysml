// `analysis case` and `verification case` are read as the analysis and
// verification case kinds with both words kept as the keyword written; the
// productions spell `analysis def`, `analysis`, `verification def` and
// `verification` alone (SysML v2 8.2.2.23, 8.2.2.24).
package CaseKeywords {
    analysis case def AD;
    verification case def VD;
    analysis case ad : AD;
    verification case vd : VD;
    analysis def AD2;
    verification def VD2;
    analysis ad2 : AD2;
    verification vd2 : VD2;
    use case def U;
    case def C;
}
