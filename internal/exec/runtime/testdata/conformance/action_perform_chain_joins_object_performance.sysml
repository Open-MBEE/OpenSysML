// `perform action run ::> target.go` performs `go` on the object `target` holds; an
// object whose type performs `go` already runs it once, so the caller joins that
// performance rather than starting a second, and reads what it left on the object.
package test {
    private import ScalarValues::*;

    part def Host {
        attribute count : Integer = 0;
        perform action go {
            action step { assign count := count + 1; }
            first step;
        }
    }

    action def Cfg {
        part target : Host;
        out result : Integer;
        perform action run ::> target.go;
        action read { assign result := target.count; }
        first run then read;
    }
}
