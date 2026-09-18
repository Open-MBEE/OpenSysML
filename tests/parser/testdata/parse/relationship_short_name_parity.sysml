// A short name is not a name, so a feature specialization keyword after one
// still states a relationship and must dump as its symbol does (SysML.xtext
// SubsetsKeyword, RedefinesKeyword, ReferencesKeyword, CrossesKeyword,
// DefinedByKeyword). Each pair below must have the same node shape.
part def B {
    attribute x : Real;
    attribute y : Real;
}

part def A :> B {
    attribute <s1> redefines x = 5;
    attribute <s2> :>> x = 5;

    attribute <s3> subsets y;
    attribute <s4> :> y;

    attribute <s5> references y;
    attribute <s6> ::> y;

    attribute <s7> defined by Real;
    attribute <s8> : Real;

    attribute <s9> redefines x[1];
    attribute <s10> :>> x[1];
}
