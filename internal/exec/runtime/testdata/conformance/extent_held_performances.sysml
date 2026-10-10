// An extent reaches the occurrences an object performs as well as the parts it owns:
// the action, state, connection, interface, allocation and exhibited or performed
// usages it holds are read and collected for `all T` as the parts are. Constraint,
// requirement and calc usages stay out: their read is an evaluation, not a held
// occurrence.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    action def Act;
    state def St { entry; then s0; state s0; }
    part def Car;
    connection def Lnk { end part x : Car; end part y : Car; }
    port def Pt;
    interface def Ifc { end port x : Pt; end port y : Pt; }
    allocation def Aloc { end part x : Car; end part y : Car; }
    calc def Chk { return r : Boolean = true; }
    constraint def K { true }
    requirement def Req;

    part def Host {
        part a : Car;
        part b : Car;
        port pa : Pt;
        port pb : Pt;
        action act : Act;
        state st : St;
        connection cn : Lnk connect a to b;
        interface ifc : Ifc connect pa to pb;
        allocation al : Aloc allocate a to b;
        perform action pf : Act;
        exhibit state ex : St;
        calc cc : Chk;
        constraint ck : K;
        requirement rq : Req;
    }

    part h : Host;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Act) == 2,
            size(all St) == 2,
            size(all Lnk) == 1,
            size(all Ifc) == 1,
            size(all Aloc) == 1,
            size(all Car) == 2,
            size(all Host) == 1
        );
    }
}
