// Two parts of one type each carry that type's assertions, and a validation
// tells them apart by path: the rear axle redefines its load over the limit, so
// only its verdict is violated. An inherited assertion is checked on the object
// once, an unasserted constraint usage is a definition rather than an assertion
// and goes unchecked, and a negated assertion holds when its condition is
// false. The satisfaction stated about the rear axle is about that object alone.
package test {
    private import ScalarValues::Real;

    part def Axle {
        attribute load : Real default = 50.0;
        assert constraint loadOk { load < 100.0 }
    }

    part def Body {
        attribute height : Real = 1.0;
        inv heightPositive { height > 0.0 }
    }

    part def LowBody :> Body {
        assert not constraint { height > 2.0 }
        constraint unasserted { height > 100.0 }
    }

    requirement def LoadReq {
        subject a : Axle;
        require constraint { a.load <= 120.0 }
    }

    part def Rig {
        part front : Axle;
        part rear : Axle {
            attribute :>> load = 150.0;
        }
        part body : LowBody;
    }

    part rig : Rig;
    requirement rearLoad : LoadReq;
    satisfy rearLoad by rig.rear;
}
