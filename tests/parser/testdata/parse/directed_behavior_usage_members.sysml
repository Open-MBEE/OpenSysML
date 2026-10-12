// A direction prefixes every usage kind a behavior body admits, not only the
// item, part and attribute parameters (SysML.xtext RefPrefix via
// OccurrenceUsagePrefix), so `out concern c : C;` declares a concern parameter.
package DirectedBehaviorUsageMembers {
    concern def C;
    requirement def R;
    use case def U;
    verification def V;
    analysis def N;
    case def K;
    constraint def Q;
    view def W;
    viewpoint def P;
    rendering def G;
    calc def F;
    item def I;
    action def Elicit {
        in concern problem : C;
        inout concern needs : C;
        out requirement spec : R {
            subject s : I;
        }
        out use case uses[*] : U;
        out verification checks[*] : V;
        out ref analysis study : N;
        out case k : K;
        in constraint rule : Q;
        in view w : W;
        in viewpoint p : P;
        in rendering g : G;
        in calc f : F;
        in state st;
        out item product : I;
        action refine {
            in concern needs : C = Elicit::needs;
            out requirement spec : R = Elicit::spec;
        }
    }
}
