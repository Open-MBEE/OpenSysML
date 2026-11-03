// The chain redefinition a satisfy usage declares on its satisfying feature holds
// on an object of that feature which exists before the check (a REPL
// `%instantiate`, or an object a caller hands to the check) as it does on a
// fresh one: the check is what the chain's domain is, not the object's birth.
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
