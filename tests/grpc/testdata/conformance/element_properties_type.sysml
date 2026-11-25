// The Type metaclass family read by specification name: abstractness,
// specialization, owned and inherited features, directed features and a
// conjugated port.
package TypeFamily {
    private import ScalarValues::*;
    port def Cmd {
        in attribute level : Integer;
        out attribute ack : Boolean;
    }
    abstract part def Vehicle {
        port cmd : Cmd;
        attribute mass : Real;
    }
    part def Car :> Vehicle {
        port reply : ~Cmd;
        part wheel : Wheel[4];
    }
    part def Wheel;
}
