package M {
    private import ScalarValues::Real;

    part def Sub {
        attribute volume : Real;
    }

    part def Ctx {
        part subsystem : Sub[*];
        part required : Sub[2..*];
        part unbounded : Sub[*..*];
        part renamed : Sub[*] :>> subsystem;
        attribute totalVolume : Real = sum(subsystem.volume);
    }

    part ctx : Ctx {
        part a : Sub :> subsystem {
            attribute :>> volume = 2.0;
        }
    }
}
