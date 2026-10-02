package Demo {
    private import ScalarValues::*;
    attribute def Sig;
    part rx;
    action def NoSucc {
        attribute c : Integer := 0;
        action a { assign c := c + 1; }
        action b { assign c := c + 10; }
        ref action r;
        abstract action x;
        send new Sig() to rx;
        assign c := c + 100;
        accept Sig;
    }
}
