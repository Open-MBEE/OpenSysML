// Control: the same satisfaction redefining the requirement's own `target` instead of a
// chain holds today, so the satisfaction path itself is sound and only a chained
// target was lost.
package ChainProbe {
    private import ScalarValues::Integer;
    part def Vehicle {
        attribute fuel : Integer default 0;
    }
    part vehicle1 : Vehicle;
    requirement def Full {
        subject vehicle : Vehicle;
        attribute target : Integer default 1300;
        require constraint { vehicle.fuel == target }
    }
    satisfy requirement check : Full by vehicle1 {
        attribute redefines target = 0;
    }
}
