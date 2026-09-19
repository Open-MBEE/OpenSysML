// Invalid: 'region' is an OpenSysML invention; the pinned grammar spells concurrency 'state ... parallel' (SysML.xtext StateDefBody).
package X04OrthogonalRegion {
    state def S {
        region r1 {
            state a;
        }
        region r2 {
            state b;
        }
    }
}
