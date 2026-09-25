// A literal is an occurrence of its enumeration, so the attributes it carries
// are readable through it, each literal holding its own values. A feature the
// enumeration declares is read through an enum-typed feature as well, since the
// literal redefining it is the value that feature holds.
package test {
    enum def Level {
        attribute n : ScalarValues::Integer;

        low {
            :>> n = 1;
            attribute label = "low";
        }
        high {
            :>> n = 9;
            attribute label = "high";
        }
    }

    part def Sensor {
        attribute l : Level = Level::high;
        attribute highN = Level::high.n;
        attribute lowN = Level::low.n;
        attribute highLabel = Level::high.label;
        attribute readThroughSlot = l.n;
    }
}
