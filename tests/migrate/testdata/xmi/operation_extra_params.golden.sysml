part def Boiler {
    attribute level : ScalarValues::Real default = 0.0;
    action def Adjust {
        in ref context : Boiler[1];
        in x : ScalarValues::Real[0..1];
        in y : ScalarValues::Real[0..1];
        first start then 'set level';
        action 'set level' {
            in value[0..1];
            if value->SequenceFunctions::notEmpty() {
                assign context.level := value;
            }
        }
        bind 'set level'.value = x;
    }
    action adjust : Adjust;
    action def Drive {
        in ref context : Boiler[1];
        /* not migrated: ControlFlow (_drE) — the edge leads to 'adjust', which never fires: no value reaches its input pin 'target' */
        action adjust : Adjust { in ref :>> context = Drive::context; in x[1]; in y[1]; }
    }
}
action def Reach {
    first start then adjust;
    action adjust : Boiler::Adjust;
}
