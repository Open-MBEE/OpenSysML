// The feature memberships a tree draws between its nodes: a port nested in a
// part definition is a non-composite feature, so its membership is drawn with
// a hollow diamond at the definition and its typing as an edge of its own from
// the port to the port definition; the parts typed by the definition draw the
// inherited port under themselves the same way.
package A {
    port def APort;

    part def APart {
        port aPort : APort;
    }

    interface def AInterface {
        end port1 : APort;
        end port2 : APort;
    }

    part part1 : APart;
    part part2 : APart;
    interface interface1 : AInterface connect part1.aPort to part2.aPort;
}

package MembershipViews {
    view structure {
        expose A::APart;
        expose A::APort;
        expose A::part1;
        expose A::part2;
    }
}
