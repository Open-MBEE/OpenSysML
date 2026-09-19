package test {
    private import ScalarValues::*;

    // A steering law: the thrust a guidance program commands is split between
    // downrange and vertical by the altitude fraction remaining.
    calc def Steering {
        in h : Real;
        out ax = 2000.0 * (1.0 - h / 15000.0);
        out ah = 2000.0 * (h / 15000.0);
    }

    // The right-hand side of a lunar-descent state [x, h, vx, vh, m]. It reads
    // its accelerations from a nested usage of the steering law, so one read of
    // its outputs must see one binding of that usage too.
    calc def Rhs {
        in x : Real;
        in h : Real;
        in vx : Real;
        in vh : Real;
        in m : Real;
        calc s : Steering { in h = h; }
        out dx = vx;
        out dh = vh;
        out dvx = s.ax / m;
        out dvh = s.ah / m - 1.62;
        out dm = 0.0 - 1.0;
    }

    // Fixed-step RK4 over the state, one step per element of `1..n`. Each stage
    // is a body-local usage of Rhs whose five outputs are read after the stage
    // before it has already updated nothing: the state features are assigned
    // only once all four stages have been read, and each stage's inputs stay
    // bound to the state its iteration started from.
    calc def Propagate {
        in n : Integer;
        attribute dt : Real = 1.0;
        attribute x : Real = 0.0;
        attribute h : Real = 15000.0;
        attribute vx : Real = 1600.0;
        attribute vh : Real = 0.0;
        attribute m : Real = 1000.0;

        for i in 1..n {
            calc k1 : Rhs { in x = x; in h = h; in vx = vx; in vh = vh; in m = m; }
            calc k2 : Rhs {
                in x = x + 0.5 * dt * k1.dx;
                in h = h + 0.5 * dt * k1.dh;
                in vx = vx + 0.5 * dt * k1.dvx;
                in vh = vh + 0.5 * dt * k1.dvh;
                in m = m + 0.5 * dt * k1.dm;
            }
            calc k3 : Rhs {
                in x = x + 0.5 * dt * k2.dx;
                in h = h + 0.5 * dt * k2.dh;
                in vx = vx + 0.5 * dt * k2.dvx;
                in vh = vh + 0.5 * dt * k2.dvh;
                in m = m + 0.5 * dt * k2.dm;
            }
            calc k4 : Rhs {
                in x = x + dt * k3.dx;
                in h = h + dt * k3.dh;
                in vx = vx + dt * k3.dvx;
                in vh = vh + dt * k3.dvh;
                in m = m + dt * k3.dm;
            }
            assign x := x + dt / 6.0 * (k1.dx + 2.0 * k2.dx + 2.0 * k3.dx + k4.dx);
            assign h := h + dt / 6.0 * (k1.dh + 2.0 * k2.dh + 2.0 * k3.dh + k4.dh);
            assign vx := vx + dt / 6.0 * (k1.dvx + 2.0 * k2.dvx + 2.0 * k3.dvx + k4.dvx);
            assign vh := vh + dt / 6.0 * (k1.dvh + 2.0 * k2.dvh + 2.0 * k3.dvh + k4.dvh);
            assign m := m + dt / 6.0 * (k1.dm + 2.0 * k2.dm + 2.0 * k3.dm + k4.dm);
        }

        return : Real = h;
    }
}
