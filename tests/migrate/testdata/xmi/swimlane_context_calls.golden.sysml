part def Engine {
    attribute rpm : ScalarValues::Real;
    action def Tune {
        in ref context : Engine;
        action 'set rpm' {
            in value;
            assign context.rpm := value;
        }
    }
    action tune : Tune;
}
part def Host {
    attribute hits : ScalarValues::Real default = 0.0;
    part engine : Engine;
    action hit {
        action 'mark hit' {
            in value;
            assign hits := value;
        }
    }
    action def Run {
        in ref context : Host;
        first start then hit;
        perform action hit ::> context.hit;
        first hit then tune;
        action tune : Engine::Tune { in ref :>> context = Run::context.engine; }
        first tune then check;
        perform action check ::> context.hit;
        /* partition 'host' represents Host: hit, tune */
        /* partition 'engine' represents Host::engine: check */
    }
}
action def Else {
    first start then run;
    action run {
        /* not migrated: CallBehaviorAction 'run' — the behavior acts on a Host through its parameter context, which is left unbound: the caller acts on no object; v1 runs Host::Run on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing */
    }
    first run then tune;
    action tune {
        /* not migrated: CallBehaviorAction 'tune' — the behavior acts on a Engine through its parameter context, which is left unbound: the caller acts on no object; v1 runs Engine::Tune on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing */
    }
}
