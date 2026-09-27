part def Engine {
    attribute rpm : ScalarValues::Real;
    action def Tune {
        in ref context : Engine;
        action 'set rpm' {
            in value;
            assign context.rpm := value;
        }
    }
    action tune : Tune { in ref :>> context = this; }
}
part def Host {
    attribute hits : ScalarValues::Real default = 0.0;
    part engine : Engine;
    action def Hit {
        in ref context : Host;
        action 'mark hit' {
            in value;
            assign context.hits := value;
        }
    }
    action def Run {
        in ref context : Host;
        first start then hit;
        action hit : Hit { in ref :>> context = Run::context; }
        first hit then tune;
        action tune : Engine::Tune { in ref :>> context = Run::context.engine; }
        first tune then check;
        action check : Hit { in ref :>> context = Run::context; }
        /* partition 'host' represents Host: hit, tune */
        /* partition 'engine' represents Host::engine: check */
    }
}
