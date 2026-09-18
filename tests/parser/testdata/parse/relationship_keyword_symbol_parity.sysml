// Each pair below writes one feature specialization twice: with its keyword and
// with its symbol (SysML.xtext SubsetsKeyword, RedefinesKeyword,
// ReferencesKeyword, CrossesKeyword, DefinedByKeyword). The two members of a
// pair must dump as the same node shape.
part def B {
    attribute x : Real;
    attribute y : Real;
}

part def A :> B {
    redefines x;
    :>> x;

    redefines x : Real;
    :>> x : Real;

    redefines x[1];
    :>> x[1];

    redefines x = 5;
    :>> x = 5;

    subsets y;
    :> y;

    subsets B::y;
    :> B::y;

    references y;
    ::> y;

    crosses y;
    => y;

    defined by Real;
    : Real;

    attribute redefines x;
    attribute :>> x;

    private subsets y;
    private :> y;
}
