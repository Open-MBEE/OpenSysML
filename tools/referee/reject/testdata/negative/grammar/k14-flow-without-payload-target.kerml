// Invalid: A flow requires both of its ends (KerML.xtext:996 Flow).
class K14A {
    feature a;
    feature b;
}
flow f from K14A::a to ;
