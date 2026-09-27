part def Boiler {
    attribute level : ScalarValues::Real default = 0.0;
    action def Adjust {
        in x : ScalarValues::Real;
        in y : ScalarValues::Real;
        in ref context : Boiler;
        first start then 'set level';
        action 'set level' {
            in value;
            assign context.level := value;
        }
        bind 'set level'.value = x;
    }
    action adjust : Adjust { in x; in y; in ref :>> context = this; }
}
