// The extent is of the type, not of what the expression can see (KerML 1.0 §7.4.9.2): `all Car`
// taken in `Other` reaches `Gaps::car`, whose package `Other` imports, and `Unrelated::truck`,
// declared in a package nothing imports, alike — in document then declaration order.
package Gaps {
    part def Car;
    part car : Car;
}

package Unrelated {
    part truck : Gaps::Car;
}

package Other {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import Gaps::*;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Car) == 2,
            (all Car)#(1) === car,
            (all Car)#(2) === Unrelated::truck
        );
    }
}
