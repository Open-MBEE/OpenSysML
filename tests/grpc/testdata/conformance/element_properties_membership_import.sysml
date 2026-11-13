// The Membership and Import metaclass families read by specification name: a
// namespace import, a membership import, a private member and an alias.
package Shared {
    part def Pump;
    part def Valve;
}
package Plant {
    public import Shared::Pump;
    private import Shared::*;
    part pump : Pump;
    private part spare : Valve;
    alias mainPump for pump;
}
