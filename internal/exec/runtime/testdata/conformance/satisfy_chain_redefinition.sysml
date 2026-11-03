// A chain redefinition a satisfy usage declares on its satisfying feature
// (`attribute redefines vehicle1.fuel = 1300;`) holds while the requirement is
// checked against that feature's object (KerML 1.1 §7.3.4.5, §7.3.4.6; SysML v2
// §7.21.4), so the requirement reads the redefined value, not the part's own.
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
        attribute redefines vehicle1.fuel = 1300;
    }
}
