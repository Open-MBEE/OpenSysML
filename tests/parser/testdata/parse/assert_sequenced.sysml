package AssertSequenced {
    private import ScalarValues::*;
    action check {
        attribute level : Integer = 0;
        action raise { assign level := 3; }
        assert constraint raised { level == 3 }
        assert not constraint high { level > 3 }
        first start then raise;
        first raise then raised;
        first raised then high;
        first high then done;
    }
}
