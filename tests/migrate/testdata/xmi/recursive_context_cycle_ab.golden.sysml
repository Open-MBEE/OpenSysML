part def Host {
    attribute k : ScalarValues::Integer default = 0;
    action def Runner {
        in ref context : Host[1];
        first start then run;
        action run : A { in ref :>> context = Runner::context; }
        first run then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def A {
        in ref context : Host[1];
        first start then b;
        action b : B { in ref :>> context = A::context; }
        first b then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def B {
        in ref context : Host[1];
        first start then a;
        action a : A { in ref :>> context = B::context; }
        first a then one;
        action one {
            out result[1] = 1;
        }
        first one then 'set k';
        action 'set k' {
            in value[1];
            assign context.k := value;
        }
        first 'set k' then final;
        action final terminate;
        flow one.result to 'set k'.value;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    state def Machine {
        in ref context : Host[1];
        entry; then A;
        state A {
            do action : Host::A { in ref :>> context = Machine::context; }
        }
        state B {
            do action : Host::B { in ref :>> context = Machine::context; }
        }
        transition first A then B;
    }
}
