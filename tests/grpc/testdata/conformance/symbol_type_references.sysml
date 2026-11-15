package DerivedTypes {
    part def T;
    part def V;
    part def Owner {
        part q : T;
        part pSub :> q;
        ref part pRef ::> q;
        part pBoth : V :> q;
        attribute a : ScalarValues::Real;
    }
    part def SubOwner :> Owner {
        part :>> q;
    }
    part tuned : Owner {
        attribute :>> a = 3.5;
    }
}
