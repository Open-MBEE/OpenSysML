// The object a satisfaction supplies with `by` is held to the subject's
// effective declaration: a derived requirement redefining the subject as
// `Ship[1]` accepts one Ship the inherited `Ship[2]` alone would refuse.
package test {
    private import ScalarValues::Real;
    private import RealFunctions::sum;

    part def Ship { attribute hullMass : Real; }
    part ship : Ship { attribute :>> hullMass = 1000.0; }

    requirement def PairLimit {
        subject pair : Ship[2];
        require constraint { sum(pair.hullMass) < 8000.0 }
    }
    requirement def OneOfPairLimit :> PairLimit { subject pair : Ship[1] :>> pair; }
    requirement oneOfPair : OneOfPairLimit;

    part context {
        assert satisfy oneOfPair by ship;
    }
}
