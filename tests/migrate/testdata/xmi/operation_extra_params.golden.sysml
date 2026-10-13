occurrence def Boiler {
    attribute level : ScalarValues::Real default = 0.0;
    action adjust {
        in x : ScalarValues::Real[0..1];
        in y : ScalarValues::Real[0..1];
        first start then 'set level';
        action 'set level' {
            in value[0..1];
            if value->SequenceFunctions::notEmpty() {
                assign level := value;
            }
        }
        bind 'set level'.value = x;
    }
    action drive {
        /* not migrated: ControlFlow (_drE) — the edge leads to 'adjust', which never fires: no value reaches its input pin 'target' */
        ref action adjust;
    }
}
action def Reach {
    first start then adjust;
    action adjust;
}
