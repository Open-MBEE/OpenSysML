package Demo {
    private import ScalarValues::*;
    attribute def Signal { attribute value : Integer; }
    action def A {
        out result : Integer = 0;
        first start;
        then send new Signal(value = 7) to self;
        then accept msg : Signal;
        then assign result := msg.value;
        then if result == 0 {
            if result == 0 {
                assign result := 1;
            } else {
                assign result := 2;
            }
        } else {
            assign result := 3;
        }
        then while result < 4 {
            assign result := result + 1;
        } until result == 4;
        then loop {
            assign result := result + 1;
        } until result == 5;
        then for i in (1, 2, 3) {
            assign result := result + i;
        }
        then terminate message;
        if result == 0 then done;
        else done;
    }
}
