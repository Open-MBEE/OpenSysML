package S {
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

    part config : Config;
    part direct : Part;

    requirement r1 : MassReq {
        attribute :>> m = direct.mass;
    }
    satisfy r1 by direct;

    requirement r2 : MassReq {
        attribute :>> m = config.child.mass;
    }
    satisfy r2 by config.child;
}
