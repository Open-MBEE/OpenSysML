package M {
    private import ScalarValues::*;
    attribute def AD { attribute q : Real; }
    enum def Color { red; green; }
    part def P {
        attribute a : AD;
        ref part r;
        part c;
        port pp : PD;
        event occurrence ev;
    }
    port def PD {
        attribute v : Real;
        port sub : PD2;
    }
    port def PD2;
    action def Act { first start; then fork f; then a1; then a2; join j; action a1; action a2; }
}
