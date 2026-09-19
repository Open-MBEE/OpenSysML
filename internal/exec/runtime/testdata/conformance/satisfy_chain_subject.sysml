// A satisfaction assertion whose `by` operand is a feature chain: the subject is
// the nested feature the chain ends in, and its object is reached through the
// objects of the features before it — the occurrence `config` is materialized,
// then the `child` it holds — so the requirement reads that nested object's
// values. Chains of depth two and three, and one through a part typed by a
// definition holding its own nested part, resolve alike.
package test {
    private import ScalarValues::Real;

    part def Part {
        attribute mass : Real = 5.0;
    }

    requirement def MassReq {
        subject subj : Part;
        attribute m : Real;
        require constraint { m < 10.0 }
    }

    part def Config {
        part child : Part;
    }

    part def Assembly {
        part config : Config;
    }

    part def Housing {
        part inner : Part {
            attribute :>> mass = 50.0;
        }
    }

    part def Rig {
        part housing : Housing;
    }

    part config : Config;
    part direct : Part;
    part assembly : Assembly;
    part rig : Rig;

    requirement r1 : MassReq {
        attribute :>> m = direct.mass;
    }
    satisfy r1 by direct;

    requirement r2 : MassReq {
        attribute :>> m = config.child.mass;
    }
    satisfy r2 by config.child;

    requirement r3 : MassReq {
        attribute :>> m = assembly.config.child.mass;
    }
    satisfy r3 by assembly.config.child;

    requirement r4 : MassReq {
        attribute :>> m = rig.housing.inner.mass;
    }
    satisfy r4 by rig.housing.inner;
}
