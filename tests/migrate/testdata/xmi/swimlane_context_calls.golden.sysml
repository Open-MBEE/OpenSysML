part def Engine {
    attribute rpm : ScalarValues::Real;
    action tune {
        action 'set rpm' {
            in value;
            assign this.rpm := value;
        }
    }
}
part def Host {
    attribute hits : ScalarValues::Real default = 0.0;
    part engine : Engine;
    action hit {
        action 'mark hit' {
            in value;
            assign this.hits := value;
        }
    }
    action run {
        first start then hit;
        perform action hit ::> Host::hit;
        first hit then tune;
        perform action tune ::> Host::engine.tune;
        first tune then check;
        perform action check ::> Host::hit;
        /* partition 'host' represents Host: hit, tune */
        /* partition 'engine' represents Host::engine: check */
    }
}
action def Else {
    in ref context : Host;
    first start then run;
    perform action run ::> context.run;
    first run then tune;
    perform action tune ::> context.engine.tune;
}
