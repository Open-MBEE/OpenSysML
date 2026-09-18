// A constraint body is a calculation body (SysML.xtext CalculationBody), so it
// carries the successions of an action body: `first a then b;`, a member-attached
// `then`, a `then` edge, a named succession, and `else` after a guarded branch.
constraint def Ordered {
    action a;
    action b;
    first a then b;
    action c;
    then action d;
    then c;
    succession s first a then c;
    action e;
    if a then b;
    else d;
    true
}
part def Tank {
    constraint c {
        action a;
        action b;
        first a then b;
        a and b
    }
}
requirement def R {
    require constraint q { action a; action b; first a then b; }
    assume constraint { action a; then action b; then a; }
    assume constraint { action a; action b; if true then a; else b; }
}
part def Pump {
    assert constraint c { action a; action b; first a then b; true }
}
// A constraint body that declares parameters and a calculation body carry them too.
constraint def Parameterised {
    in x : ScalarValues::Boolean;
    action a;
    action b;
    first a then b;
    then action c;
    x
}
calc def K {
    action a;
    action b;
    first a then b;
    then action c;
    else b;
    1
}
