package Lib {
    attribute def Gauge;
    attribute limit : Gauge;
}
package Scopes {
    public import Lib::*;
    part def D {
        attribute xs;
        attribute one = xs->collect({ in limit; limit });
        attribute two = xs->select({ in Gauge; Gauge });
        attribute outside = limit;
        attribute g : Gauge;
    }
}
