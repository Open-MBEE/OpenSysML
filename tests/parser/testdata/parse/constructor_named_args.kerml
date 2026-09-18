package ConstructorNamedArgs {
    class T;
    class C { feature x : T; feature y : T; }
    feature t : T;
    feature positional : C = new C(t, t);
    feature named : C = new C(x = t, y = t);
    feature empty : C = new C();
}
