// The outcome of an action holds, under `part.attribute`, what the one object each
// of its own parts denotes holds when it completes — the values a performed behavior
// left on `target` — so a run configuration's observables are read off its target;
// a part of several objects is not spelled, having no one value per attribute.
package test {
    private import ScalarValues::*;

    part def Probe {
        attribute total : Real = 0.0;
        attribute tries : Integer = 0;
        perform action measure {
            action step { assign total := total + 2.5; assign tries := tries + 1; }
            first step;
        }
    }

    action def Cfg {
        part target : Probe;
        part spares : Probe[2];
        perform action run ::> target.measure;
    }
}
